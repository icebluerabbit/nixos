// Package metrics owns the Prometheus registry and exports mousetrap's metrics by
// PULL: a custom collector reads the state snapshot and summarizes the store ledgers
// (announces, grabs, spends, bonuses) on each scrape. Nothing is pushed from the
// hot path — the ledgers are the source of truth and the metrics derive from them.
package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"homelab/mousetrap/internal/state"
	"homelab/mousetrap/internal/store"
)

// Metrics owns the registry and serves /metrics. Most metrics are pulled by the
// collector; grabDur and mamErr are pushed from the hot path (timing/errors can't be
// derived from the ledgers).
type Metrics struct {
	reg     *prometheus.Registry
	grabDur prometheus.Histogram
	mamErr  *prometheus.CounterVec
}

// New registers the collector over state + store on reg. window is the
// in-flight reconciliation window (for the current-in-flight gauges).
//
// The registry is supplied rather than created here so that the series reach
// whatever lib/app built for the process: one registry backs both this tool's
// own /metrics route and the framework's optional --metrics.addr endpoint.
func New(reg *prometheus.Registry, st *state.State, db *store.Store, window time.Duration) *Metrics {
	m := &Metrics{
		reg: reg,
		grabDur: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "mousetrap_grab_duration_seconds",
			Help:    "grab fetch + client-add duration",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 15},
		}),
		mamErr: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mousetrap_mam_errors_total",
			Help: "MAM API errors by operation",
		}, []string{"op"}),
	}
	for _, op := range []string{"loaduser", "search", "download", "bonus", "buy"} {
		m.mamErr.WithLabelValues(op) // pre-init so increase() sees the 0→N edge
	}
	reg.MustRegister(&collector{st: st, store: db, window: window}, m.grabDur, m.mamErr)
	return m
}

// ObserveGrab records a grab's fetch+add duration.
func (m *Metrics) ObserveGrab(d time.Duration) { m.grabDur.Observe(d.Seconds()) }

// MAMError increments the error counter for a MAM operation (loaduser, search,
// download, bonus, buy).
func (m *Metrics) MAMError(op string) { m.mamErr.WithLabelValues(op).Inc() }

// Handler serves the Prometheus exposition endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

type collector struct {
	st     *state.State
	store  *store.Store
	window time.Duration
}

// account/current-value gauges
var (
	descRatio      = prometheus.NewDesc("mam_ratio", "account ratio", nil, nil)
	descSeedBonus  = prometheus.NewDesc("mam_seedbonus", "bonus points", nil, nil)
	descUploaded   = prometheus.NewDesc("mam_uploaded_bytes", "uploaded bytes", nil, nil)
	descDownloaded = prometheus.NewDesc("mam_downloaded_bytes", "downloaded bytes", nil, nil)
	descBuffer     = prometheus.NewDesc("mam_buffer_bytes", "uploaded - downloaded", nil, nil)
	descUnsatCount = prometheus.NewDesc("mam_unsat_count", "unsatisfied torrents", nil, nil)
	descUnsatLimit = prometheus.NewDesc("mam_unsat_limit", "unsatisfied limit for class", nil, nil)
	descSeeding    = prometheus.NewDesc("mam_seeding_count", "seeding torrents", nil, nil)
	descSatisfied  = prometheus.NewDesc("mam_satisfied_count", "satisfied snatches", nil, nil)
	descSeedUnsat  = prometheus.NewDesc("mam_seed_unsat_count", "seeding but unsatisfied", nil, nil)
	descLeeching   = prometheus.NewDesc("mam_leeching_count", "leeching torrents", nil, nil)
	descHnr        = prometheus.NewDesc("mam_hnr_count", "hit-and-run torrents", nil, nil)
	descSeedHnr    = prometheus.NewDesc("mam_seed_hnr_count", "seeding hit-and-run", nil, nil)
	descInactHnr   = prometheus.NewDesc("mam_inact_hnr_count", "inactive hit-and-run", nil, nil)
	descWedges     = prometheus.NewDesc("mam_wedges", "freeleech wedges", nil, nil)
	descClass      = prometheus.NewDesc("mam_class_info", "account class (value always 1)", []string{"classname"}, nil)
	descRefreshTS  = prometheus.NewDesc("mam_refresh_timestamp", "unix ts of the last account snapshot refresh", nil, nil)
	descBonusCount = prometheus.NewDesc("mam_bonus_history_count", "mirrored bonus ledger rows", nil, nil)

	// current optimistic in-flight (over the reconciliation window)
	descInflightCount  = prometheus.NewDesc("mousetrap_inflight_count", "in-flight grabs (unsat)", nil, nil)
	descInflightRatio  = prometheus.NewDesc("mousetrap_inflight_ratio_bytes", "in-flight ratio-costing bytes", nil, nil)
	descInflightWedges = prometheus.NewDesc("mousetrap_inflight_wedges", "in-flight wedges", nil, nil)
	descInflightPoints = prometheus.NewDesc("mousetrap_inflight_points", "in-flight bonus points spent", nil, nil)
)

// ledger-derived counters
var (
	descAnnTotal   = prometheus.NewDesc("mousetrap_announces_total", "MAM announces received", nil, nil)
	descAnnType    = prometheus.NewDesc("mousetrap_announces_by_type_total", "by MAM media type", []string{"type"}, nil)
	descAnnCat     = prometheus.NewDesc("mousetrap_announces_by_category_total", "by category (multi-valued; sums above the announce total)", []string{"category"}, nil)
	descAnnLang    = prometheus.NewDesc("mousetrap_announces_by_language_total", "by language", []string{"language"}, nil)
	descAnnFT      = prometheus.NewDesc("mousetrap_announces_by_filetype_total", "by filetype (multi-valued; sums above the announce total)", []string{"filetype"}, nil)
	descAnnFic     = prometheus.NewDesc("mousetrap_announces_by_fiction_total", "by MAM fiction flag; NULL on pre-2026-08-12 rows and search results", []string{"fiction"}, nil)
	descAnnVIP     = prometheus.NewDesc("mousetrap_announces_vip_total", "by VIP flag", []string{"vip"}, nil)
	descAnnFL      = prometheus.NewDesc("mousetrap_announces_freeleech_total", "by freeleech flag", []string{"freeleech"}, nil)
	descAnnBytes   = prometheus.NewDesc("mousetrap_announce_bytes_total", "announced bytes by type", []string{"type"}, nil)
	descAnnSize    = prometheus.NewDesc("mousetrap_announce_size_bytes", "announced torrent size distribution", []string{"type"}, nil)
	descWouldAcc   = prometheus.NewDesc("mousetrap_would_accept_total", "layer-1 accept decisions", []string{"decision", "reason"}, nil)
	descWouldGrab  = prometheus.NewDesc("mousetrap_would_grab_total", "layer-2 grab decisions", []string{"decision", "reason"}, nil)
	descGrabAction = prometheus.NewDesc("mousetrap_grab_action_total", "grab action results", []string{"result"}, nil)
	descGrabSource = prometheus.NewDesc("mousetrap_grabs_by_source_total", "grabs by origin", []string{"source"}, nil)
	descGrabBytes  = prometheus.NewDesc("mousetrap_grab_bytes_total", "bytes of successfully-added grabs", nil, nil)
	descWedgeSaved = prometheus.NewDesc("mousetrap_wedge_saved_bytes_total", "ratio bytes saved via wedges", nil, nil)
	descWedgeSkip  = prometheus.NewDesc("mousetrap_wedge_skipped_bytes_total", "ratio bytes avoided by skipping proactive-wedge releases with no wedge available", nil, nil)
	descSpends     = prometheus.NewDesc("mousetrap_spends_total", "outgoing spends by reason", []string{"reason"}, nil)
	descSpendType  = prometheus.NewDesc("mousetrap_spends_by_type_total", "outgoing spends by type", []string{"type"}, nil)
	descSpendSum   = prometheus.NewDesc("mousetrap_spend_amount_total", "spent amount by type", []string{"type"}, nil)
	descBonusType  = prometheus.NewDesc("mousetrap_bonus_received_total", "received bonus rows by type", []string{"type"}, nil)
	descBonusSum   = prometheus.NewDesc("mousetrap_bonus_amount_total", "received bonus amount by type (signed)", []string{"type"}, nil)
)

// Describe is intentionally empty — this is an unchecked collector; all metrics are
// produced dynamically in Collect from the current ledger state.
func (c *collector) Describe(chan<- *prometheus.Desc) {}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	gauge := func(d *prometheus.Desc, v float64, labels ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v, labels...)
	}
	counter := func(d *prometheus.Desc, v float64, labels ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.CounterValue, v, labels...)
	}
	// emit1 turns a one-key grouped aggregate into per-label counters.
	emit1 := func(d *prometheus.Desc, rows []store.Count1, err error, mapKey func(string) string) {
		if err != nil {
			return
		}
		for _, r := range rows {
			k := r.Key
			if mapKey != nil {
				k = mapKey(k)
			}
			counter(d, float64(r.N), k)
		}
	}
	emit2 := func(d *prometheus.Desc, rows []store.Count2, err error) {
		if err != nil {
			return
		}
		for _, r := range rows {
			counter(d, float64(r.N), r.A, r.B)
		}
	}

	// ── account snapshot (state) ──
	a := c.st.Account()
	gauge(descRatio, a.Ratio)
	gauge(descSeedBonus, float64(a.SeedBonus))
	gauge(descUploaded, float64(a.UploadedBytes))
	gauge(descDownloaded, float64(a.DownloadedBytes))
	gauge(descBuffer, float64(a.Buffer()))
	gauge(descUnsatCount, float64(a.UnsatCount))
	gauge(descUnsatLimit, float64(a.UnsatLimit))
	gauge(descSeeding, float64(a.SeedingCount))
	gauge(descSatisfied, float64(a.SatisfiedCount))
	gauge(descSeedUnsat, float64(a.SeedUnsatCount))
	gauge(descLeeching, float64(a.LeechingCount))
	gauge(descHnr, float64(a.HnrCount))
	gauge(descSeedHnr, float64(a.SeedHnrCount))
	gauge(descInactHnr, float64(a.InactHnrCount))
	gauge(descWedges, float64(a.Wedges))
	if a.ClassName != "" {
		gauge(descClass, 1, a.ClassName)
	}
	if ts := c.st.LastRefresh(); !ts.IsZero() {
		gauge(descRefreshTS, float64(ts.Unix()))
	}

	// ── current in-flight (store, over the window) ──
	if f, err := c.store.InFlightSince(ctx, time.Now().Add(-c.window).Unix()); err == nil {
		gauge(descInflightCount, float64(f.Count))
		gauge(descInflightRatio, float64(f.RatioBytes))
		gauge(descInflightWedges, float64(f.Wedges))
		gauge(descInflightPoints, float64(f.Points))
	}
	if n, err := c.store.BonusTotal(ctx); err == nil {
		gauge(descBonusCount, float64(n))
	}

	// ── announces ledger ──
	if n, err := c.store.AnnouncesTotal(ctx); err == nil {
		counter(descAnnTotal, float64(n))
	}
	r, err := c.store.AnnouncesBy(ctx, "media_type")
	emit1(descAnnType, r, err, nil)
	r, err = c.store.AnnounceCategories(ctx)
	emit1(descAnnCat, r, err, nil)
	r, err = c.store.AnnounceLanguages(ctx)
	emit1(descAnnLang, r, err, nil)
	r, err = c.store.AnnounceFiletypes(ctx)
	emit1(descAnnFT, r, err, nil)
	r, err = c.store.AnnouncesBy(ctx, "fiction")
	emit1(descAnnFic, r, err, nil)
	r, err = c.store.AnnouncesBy(ctx, "vip")
	emit1(descAnnVIP, r, err, boolLabel)
	r, err = c.store.AnnouncesBy(ctx, "freeleech")
	emit1(descAnnFL, r, err, boolLabel)
	r, err = c.store.AnnounceBytesByMediaType(ctx)
	emit1(descAnnBytes, r, err, nil)
	if hs, err := c.store.AnnounceSizeHistogram(ctx, sizeBuckets); err == nil {
		for _, h := range hs {
			ch <- prometheus.MustNewConstHistogram(descAnnSize, h.Count, h.Sum, h.Buckets, h.MediaType)
		}
	}
	r2, err := c.store.AnnouncesByAccept(ctx)
	emit2(descWouldAcc, r2, err)
	r2, err = c.store.AnnouncesByGrab(ctx)
	emit2(descWouldGrab, r2, err)

	// ── grabs ledger ──
	r, err = c.store.GrabsByResult(ctx)
	emit1(descGrabAction, r, err, nil)
	r, err = c.store.GrabsBySource(ctx)
	emit1(descGrabSource, r, err, nil)
	if n, err := c.store.GrabBytesTotal(ctx); err == nil {
		counter(descGrabBytes, float64(n))
	}
	if n, err := c.store.WedgeSavedBytes(ctx); err == nil {
		counter(descWedgeSaved, float64(n))
	}
	if n, err := c.store.SkippedWedgeBytes(ctx); err == nil {
		counter(descWedgeSkip, float64(n))
	}

	// ── spends ledger ──
	r, err = c.store.SpendsByReason(ctx)
	emit1(descSpends, r, err, nil)
	r, err = c.store.SpendsByType(ctx)
	emit1(descSpendType, r, err, nil)
	r, err = c.store.SpendSumByType(ctx)
	emit1(descSpendSum, r, err, nil)

	// ── bonuses ledger (received) ──
	r, err = c.store.BonusesByType(ctx)
	emit1(descBonusType, r, err, nil)
	r, err = c.store.BonusSumByType(ctx)
	emit1(descBonusSum, r, err, nil)
}

// sizeBuckets matches the announce-size histogram buckets the dashboard queries.
var sizeBuckets = []float64{1e5, 1e6, 1e7, 5e7, 1e8, 3e8, 1e9, 3e9}

// boolLabel maps a 0/1 integer column value to a true/false label.
func boolLabel(k string) string {
	if k == "1" {
		return "true"
	}
	return "false"
}
