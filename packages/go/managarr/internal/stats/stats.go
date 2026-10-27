// Package stats exports what the model knows as Prometheus gauges.
//
// Distinct from the reconcile metrics, which describe the PASSES — how long one
// took, whether it failed. These describe the STACK: how many torrents are
// seeding below ratio, where the acquisition funnel leaks, how much of the
// library has subtitles.
//
// Gauges rather than counters throughout, because every one of these is a
// snapshot of a table rather than an accumulating event count. managarr already
// holds the events; re-counting them as counters would drift from the rows the
// moment a backfill or a delete moved anything.
//
// Refreshed on its own ticker rather than with the reconcile. The reconcile is
// hourly because it stats every file over NFS; these are indexed aggregates
// that cost milliseconds, and pinning them to the pass would throw away every
// trend finer than an hour.
package stats

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

// query is one metric and the SQL that fills it.
//
// Table-driven because every one of these has the same shape: group by a few
// low-cardinality columns, count or sum, publish. Fifteen hand-written
// collectors would be fifteen places to get the registration wrong.
//
// The SQL must return the label columns in the order labels names them,
// followed by one numeric column. A row whose label value is NULL is rendered
// as the empty string rather than skipped — "torrents with no state" is a real
// answer and hiding it would make the totals not add up.
type query struct {
	name   string
	help   string
	labels []string
	sql    string
}

// queries is every metric this package exports.
//
// Cardinality was measured against the live database before any of these were
// written, not assumed: decisions carry 3 outcomes across 12 deciders and 2
// stages, offers 2 kinds across 10 indexers. The largest series count here is
// in the low hundreds, which is what makes a GROUP BY safe to publish at all.
var queries = []query{
	// ── acquisition funnel ────────────────────────────────────────────────
	//
	// The question this exists to answer: of everything announced, what was
	// offered, what was decided, and where did it stop. autobrr accepting
	// 129,000 releases while Sonarr rejects 80,000 is the shape of the
	// funnel, and no single service can see it.
	{
		name: "managarr_offers", help: "Offers recorded, by kind and indexer.",
		labels: []string{"kind", "indexer"},
		sql: `SELECT kind, COALESCE(indexer_name,''), count(*)
		        FROM offers GROUP BY kind, indexer_name`,
	},
	{
		name: "managarr_decisions", help: "Decisions recorded, by decider, stage and outcome.",
		labels: []string{"decider", "stage", "outcome"},
		sql: `SELECT decider, stage, outcome, count(*)
		        FROM decisions GROUP BY decider, stage, outcome`,
	},
	{
		name: "managarr_grabs", help: "Grabs recorded, by service and indexer.",
		labels: []string{"service", "indexer"},
		sql: `SELECT service_name, COALESCE(indexer_name,''), count(*)
		        FROM grabs GROUP BY service_name, indexer_name`,
	},
	{
		name: "managarr_import_failures", help: "Import failures, by service and kind.",
		labels: []string{"service", "kind"},
		sql: `SELECT service_name, kind, count(*)
		        FROM import_failures GROUP BY service_name, kind`,
	},

	// ── seeding health ────────────────────────────────────────────────────
	//
	// The only part of this model with money attached: a tracker's
	// hit-and-run rule is enforced against ratio and seeding time, and
	// nothing else in the stack reports either across every client.
	{
		name: "managarr_client_torrents", help: "Torrents held by the client, by state.",
		labels: []string{"client", "state"},
		sql: `SELECT client_name, COALESCE(state,''), count(*)
		        FROM client_torrents WHERE removed_at IS NULL
		       GROUP BY client_name, state`,
	},
	{
		name: "managarr_client_torrents_by_ratio",
		help: "Torrents held by the client, bucketed by share ratio.",
		// Bands rather than a histogram: this is a snapshot of a table, and a
		// Prometheus histogram describes observations over time. The bands are
		// where tracker rules actually sit.
		labels: []string{"client", "band"},
		sql: `SELECT client_name,
		             CASE WHEN ratio IS NULL THEN 'unknown'
		                  WHEN ratio < 0.5 THEN 'under_0.5'
		                  WHEN ratio < 1   THEN 'under_1'
		                  WHEN ratio < 2   THEN 'under_2'
		                  WHEN ratio < 5   THEN 'under_5'
		                  ELSE 'over_5' END,
		             count(*)
		        FROM client_torrents WHERE removed_at IS NULL
		       GROUP BY 1, 2`,
	},
	{
		name: "managarr_client_uploaded_bytes", help: "Bytes uploaded by the client, total.",
		labels: []string{"client"},
		sql: `SELECT client_name, COALESCE(sum(uploaded_bytes),0)
		        FROM client_torrents WHERE removed_at IS NULL GROUP BY client_name`,
	},
	{
		name: "managarr_client_downloaded_bytes", help: "Bytes downloaded by the client, total.",
		labels: []string{"client"},
		sql: `SELECT client_name, COALESCE(sum(downloaded_bytes),0)
		        FROM client_torrents WHERE removed_at IS NULL GROUP BY client_name`,
	},
	{
		name: "managarr_client_seeding_seconds", help: "Seeding time accumulated, total.",
		labels: []string{"client"},
		sql: `SELECT client_name, COALESCE(sum(seeding_time_seconds),0)
		        FROM client_torrents WHERE removed_at IS NULL GROUP BY client_name`,
	},

	// ── library ───────────────────────────────────────────────────────────
	{
		name: "managarr_contents", help: "Catalog entries, by kind.",
		labels: []string{"kind"},
		sql:    `SELECT kind, count(*) FROM contents GROUP BY kind`,
	},
	{
		name: "managarr_library_files", help: "Files present in the library, by service.",
		labels: []string{"service"},
		sql: `SELECT COALESCE(service_name,''), count(*)
		        FROM library_files WHERE removed_at IS NULL GROUP BY service_name`,
	},
	{
		name: "managarr_library_files_by_resolution",
		help: "Library files by video resolution.",
		// Joined rather than counted from media_info alone: a removed file
		// keeps its media_info row, and counting those would report a library
		// larger than the one on disk.
		labels: []string{"resolution"},
		sql: `SELECT COALESCE(m.resolution,''), count(*)
		        FROM media_info m JOIN library_files f ON f.id = m.library_file_id
		       WHERE f.removed_at IS NULL GROUP BY m.resolution`,
	},
	{
		name: "managarr_subtitle_files", help: "Subtitle files present, by language.",
		labels: []string{"language"},
		sql: `SELECT COALESCE(language_code,''), count(*)
		        FROM subtitle_files GROUP BY language_code`,
	},
	{
		name: "managarr_subtitle_wanted", help: "Subtitles still wanted, by language.",
		// The only absence in this model that can be counted. A missing
		// subtitle leaves nothing on disk, so nothing but Bazarr knows.
		labels: []string{"language"},
		sql: `SELECT COALESCE(language_code,''), count(*)
		        FROM subtitle_wanted GROUP BY language_code`,
	},

	// ── music chain ───────────────────────────────────────────────────────
	{
		name: "managarr_soulseek_downloads", help: "Soulseek downloads recorded, by state.",
		labels: []string{"state"},
		sql: `SELECT COALESCE(state,''), count(*)
		        FROM soulseek_downloads GROUP BY state`,
	},
	{
		name: "managarr_soulseek_uploads", help: "Soulseek uploads recorded, by state.",
		// What this homelab has SHARED, which exists in slskd's event log and
		// nowhere else once a transfer ages out of its API.
		labels: []string{"state"},
		sql: `SELECT COALESCE(state,''), count(*)
		        FROM soulseek_uploads GROUP BY state`,
	},
	{
		name: "managarr_beets_items", help: "Tagged music files, by whether they resolve to a catalog entry.",
		// matched is the music loop closing: beets' mb_releasegroupid is
		// Lidarr's foreignAlbumId, so an unmatched item is a file the library
		// cannot explain.
		labels: []string{"matched"},
		sql: `SELECT CASE WHEN content_id IS NULL THEN 'false' ELSE 'true' END, count(*)
		        FROM beets_items GROUP BY 1`,
	},

	// ── indexers ──────────────────────────────────────────────────────────
	{
		name: "managarr_searches", help: "Searches recorded, by indexer.",
		labels: []string{"indexer"},
		sql: `SELECT COALESCE(indexer_name,''), count(*)
		        FROM searches GROUP BY indexer_name`,
	},
	{
		name: "managarr_search_elapsed_ms_avg", help: "Mean search latency, by indexer.",
		labels: []string{"indexer"},
		sql: `SELECT COALESCE(indexer_name,''), COALESCE(avg(elapsed_ms),0)
		        FROM searches WHERE elapsed_ms IS NOT NULL GROUP BY indexer_name`,
	},
	{
		name: "managarr_indexer_grabs", help: "Grabs proxied through each indexer.",
		labels: []string{"indexer"},
		sql: `SELECT COALESCE(indexer_name,''), count(*)
		        FROM indexer_grabs GROUP BY indexer_name`,
	},

	// ── cursors ───────────────────────────────────────────────────────────
	//
	// Whether the sidecar design is doing its job. events_read that climbs by
	// the whole corpus every hour means a cursor is resetting; lag that grows
	// without bound means a pass has stopped consuming one.
	{
		name:   "managarr_sidecar_cursor_events_read",
		help:   "Events read through each sidecar cursor, cumulative.",
		labels: []string{"source"},
		sql:    `SELECT source, events_read FROM sidecar_cursors`,
	},
	{
		name:   "managarr_sidecar_cursor_lag_seconds",
		help:   "Age of the newest event read through each sidecar cursor.",
		labels: []string{"source"},
		sql: `SELECT source, EXTRACT(EPOCH FROM (now() - last_event_at))
		        FROM sidecar_cursors WHERE last_event_at IS NOT NULL`,
	},
}

// Collector refreshes every gauge from the database.
type Collector struct {
	db     *gorm.DB
	logger *slog.Logger
	gauges map[string]*prometheus.GaugeVec
	// refreshed reports the last successful refresh, so a collector that has
	// quietly stopped is visible. Without it a stale gauge is
	// indistinguishable from an unchanging one.
	refreshed prometheus.Gauge
	failures  *prometheus.CounterVec
}

// New builds a collector and registers its gauges.
func New(db *gorm.DB, logger *slog.Logger, reg prometheus.Registerer) *Collector {
	c := &Collector{
		db:     db,
		logger: logger,
		gauges: make(map[string]*prometheus.GaugeVec, len(queries)),
		refreshed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "managarr_stats_refreshed_timestamp_seconds",
			Help: "When the domain gauges were last refreshed in full.",
		}),
		failures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "managarr_stats_failures_total",
			Help: "Domain queries that returned an error, by metric.",
		}, []string{"metric"}),
	}

	for _, q := range queries {
		c.gauges[q.name] = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: q.name, Help: q.help}, q.labels)
	}

	if reg != nil {
		reg.MustRegister(c.refreshed, c.failures)
		for _, g := range c.gauges {
			reg.MustRegister(g)
		}
	}
	return c
}

// Run refreshes on a ticker until ctx is cancelled.
//
// Refreshes once immediately: a process that has just started otherwise
// exports nothing until the first tick, and a dashboard opened in that window
// shows an empty stack rather than a loading one.
func (c *Collector) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	c.Refresh(ctx)

	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.Refresh(ctx)
		}
	}
}

// Refresh runs every query and republishes the gauges.
//
// One failing query does not abandon the rest: they are independent, and a
// column renamed in one table should cost that metric rather than the whole
// dashboard.
func (c *Collector) Refresh(ctx context.Context) {
	var failed int
	for _, q := range queries {
		if err := c.refreshOne(ctx, q); err != nil {
			failed++
			c.failures.WithLabelValues(q.name).Inc()
			c.logger.WarnContext(ctx, "stats query failed",
				"metric", q.name, "err", err)
		}
	}
	if failed == 0 {
		c.refreshed.Set(float64(time.Now().Unix()))
	}
}

// refreshOne replaces one metric's series.
func (c *Collector) refreshOne(ctx context.Context, q query) error {
	rows, err := c.db.WithContext(ctx).Raw(q.sql).Rows()
	if err != nil {
		return err
	}
	// Nothing to do with a close error on a read: the rows are already scanned,
	// and rows.Err reports anything that went wrong reading them.
	defer func() { _ = rows.Close() }()

	gauge := c.gauges[q.name]
	// Reset before filling. A label combination that no longer exists — the
	// last torrent in a state, an indexer removed — would otherwise keep its
	// final value forever and read as current.
	gauge.Reset()

	for rows.Next() {
		labels := make([]string, len(q.labels))
		var value float64

		dest := make([]any, 0, len(q.labels)+1)
		for i := range labels {
			dest = append(dest, &labels[i])
		}
		dest = append(dest, &value)

		if err := rows.Scan(dest...); err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		gauge.WithLabelValues(labels...).Set(value)
	}
	return rows.Err()
}
