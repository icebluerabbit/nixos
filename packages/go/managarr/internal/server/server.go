// Package server is managarr's inbound surface: one path per event source, plus
// the endpoint the reconcile timer drives.
//
// Every request is logged and handed to the ingest package, which persists the
// sources it knows. The log line survives from the phase that built the payload
// corpus — the *arr bodies differ per event in ways the docs do not spell out,
// qBittorrent supplies substituted tokens rather than JSON, and autobrr's shape
// is whatever its action template was given — and it is still how an unparsed
// or newly-changed sender gets diagnosed.
//
// Credentials are stripped before anything is logged: this log ships to Loki,
// and several senders put a tracker passkey in the body or the query string.
//
// Every hook answers 200 regardless of what the parser made of it. A sender
// that gets a non-2xx disables itself or retries, and a body this build cannot
// parse is a defect here rather than at the sender — retrying would only repeat
// it.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"homelab/managarr/internal/redact"
)

// maxBodyBytes bounds what is read from a request. Sonarr's on_import_complete
// carries an episodeFiles array that grows with the season, and a season pack
// import is the largest body this sees; 8 MiB is far above that and still
// stops a malformed sender from exhausting memory.
const maxBodyBytes = 8 << 20

// ingestTimeout bounds one delivery's writes. Generous: a Vector batch can
// carry two dozen objects and each is a handful of statements, but it must not
// be unbounded or a stuck pool would pin goroutines for the process lifetime.
const ingestTimeout = 30 * time.Second

// Sources are the paths served under /hook/. The set is fixed rather than
// open: an unknown path is far more likely to be a misconfigured sender than a
// new integration, and silently accepting it would hide the mistake until the
// corpus turned out to have a hole in it.
var Sources = []string{
	"sonarr",
	"radarr",
	"lidarr",
	"readarr",
	"prowlarr",
	"autobrr",
	"unpackerr",
	"qbittorrent",
	"jellyfin",
	"cross-seed",
	"seasonpackarr",
	// Soulseek side of the music stack. Neither slskd nor soularr has a
	// webhook, so both arrive via their Vector journal transforms rather
	// than from the application itself — the payload is whatever the remap
	// produced, not a vendor schema.
	"slskd",
	"soularr",
	// Torrents dropped into /mnt/media/watch/<category> by hand. These
	// enter qBittorrent with no grab behind them, which is indistinguishable
	// from a cross-seed inject unless the importer says so itself.
	"qbt-watch",
}

// Deps is what the server needs. Injected so tests can supply a clock and read
// the metrics back.
type Deps struct {
	Logger *slog.Logger
	// Now is the clock, injectable so a test can assert on recorded times.
	Now func() time.Time
	// Reconcile backs POST /reconcile. A plain func rather than an
	// interface: the only shape the server needs is "do a pass, tell me if
	// it failed", and the pass reports its own counts to the log. Nil
	// disables the endpoint, which is what a deployment with no qBittorrent
	// URL gets.
	Reconcile func(ctx context.Context) error
	// Ingest persists one webhook body. A plain func for the same reason as
	// Reconcile: the server's whole interest is "here is a body, do what you
	// do with it". Nil records to the log only, which is what a deployment
	// with no database gets.
	//
	// It takes no error: a body this cannot parse is a defect in the parser or
	// a change at the sender, and neither is fixed by failing the request.
	Ingest func(ctx context.Context, source string, body []byte)
	// Cleanup removes torrents their tracker has unregistered, reporting what
	// it did. dryRun finds them and changes nothing.
	//
	// The only destructive thing this server exposes, so unlike Reconcile it
	// runs SYNCHRONOUSLY and answers with the result. A caller deleting data
	// is entitled to learn what was deleted in the same breath, and the work
	// is one API call rather than a pass over NFS.
	//
	// Nil disables the endpoint, which is what a deployment with no torrent
	// client gets.
	Cleanup func(ctx context.Context, dryRun bool) (any, error)
	// Orphans deletes library entries no torrent provides any more, reporting
	// what it did. dryRun finds them and changes nothing.
	//
	// The second destructive endpoint, and the more destructive of the two: it
	// unlinks media files, where Cleanup unlinks a torrent's copy and usually
	// leaves the bytes behind another hardlink. Same synchronous shape and same
	// dry-run default for that reason.
	//
	// Nil disables the endpoint, which is what a deployment with no *arr gets.
	Orphans func(ctx context.Context, dryRun bool) (any, error)
	// Reap removes everything else the stack has stranded — download data with
	// no torrent, dead cross-seed link trees, abandoned partials — and reports
	// the two inverse classes it cannot act on: torrents whose data is gone,
	// and library rows whose file is gone.
	//
	// dryRun finds everything and removes nothing.
	//
	// Nil disables the endpoint.
	Reap func(ctx context.Context, dryRun bool) (any, error)
	// Tags projects managarr's view onto the torrent client's tags, REPLACING
	// what is there. dryRun computes everything and writes nothing.
	//
	// Destructive in a recoverable way, unlike the other two: a wrong tag set
	// is fixed by the next pass, where a deleted file is not. It still defaults
	// to a dry run, because "replaces every tag on 7600 torrents" deserves to
	// be read before it is run.
	//
	// Nil disables the endpoint.
	Tags func(ctx context.Context, dryRun bool) (any, error)
	// Packs records which episode torrents each assembled season pack was built
	// from, by resolving the pack directory's contents to client torrent names.
	//
	// Writes only associations — it creates and deletes nothing else — so a
	// wrong row is corrected by the next pass.
	//
	// Nil disables the endpoint.
	Packs func(ctx context.Context, dryRun bool) (any, error)
}

// Server serves the hook endpoints.
type Server struct {
	deps    Deps
	metrics *metrics
	// running guards against overlapping reconcile passes.
	running atomic.Bool
}

// metrics are per-server rather than package-level so two servers in one test
// binary do not fight over a registry.
type metrics struct {
	requests *prometheus.CounterVec
	bodySize *prometheus.HistogramVec
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "managarr_hook_requests_total",
			Help: "Webhook requests received, by source and outcome.",
		}, []string{"source", "outcome"}),
		bodySize: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "managarr_hook_body_bytes",
			Help: "Received body size in bytes, by source.",
			// Webhook bodies run from a few hundred bytes (qBittorrent's
			// token line) to a season-pack import payload.
			Buckets: prometheus.ExponentialBuckets(128, 4, 8),
		}, []string{"source"}),
	}
	if reg != nil {
		reg.MustRegister(m.requests, m.bodySize)
	}
	return m
}

// New builds a server. reg may be nil, which drops the metrics.
func New(deps Deps, reg prometheus.Registerer) (*Server, error) {
	if deps.Logger == nil {
		return nil, errors.New("server: Logger is required")
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &Server{deps: deps, metrics: newMetrics(reg)}, nil
}

// Handler builds the mux. Every known source gets its own path so the source
// is known from the route rather than sniffed out of the payload — several
// senders post bodies that are indistinguishable from one another.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	for _, src := range Sources {
		mux.HandleFunc("/hook/"+src, s.record(src))
	}
	// Anything else is a misconfigured sender. 404 so it shows up as a
	// failure at the sender rather than vanishing into the log.
	mux.HandleFunc("/hook/", func(w http.ResponseWriter, r *http.Request) {
		s.deps.Logger.Warn("unknown hook path",
			"path", r.URL.Path,
			"remote", remoteHost(r),
			"known", strings.Join(Sources, ","),
		)
		s.metrics.requests.WithLabelValues("unknown", "not_found").Inc()
		http.NotFound(w, r)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok\n")
	})
	mux.HandleFunc("/reconcile", s.reconcile)
	// Where the managarr-* sidecars relay what they enriched.
	//
	// One route for all thirteen rather than a path per source, because the
	// envelope already names its source — that is the whole point of
	// normalising at the edge. A sidecar posts a batch; each event inside
	// carries its own source and kind.
	mux.HandleFunc("/ingest/sidecar", s.ingestSidecar)
	mux.HandleFunc("/cleanup", s.cleanup)
	mux.HandleFunc("/orphans", s.orphans)
	mux.HandleFunc("/reap", s.reap)
	mux.HandleFunc("/tags", s.tags)
	mux.HandleFunc("/packs", s.packs)
	return mux
}

// packs records which episodes each assembled season pack was built from.
//
// Dry run by default like the rest, though this one only ever writes
// associations: a wrong row misattributes a pack, which is worth seeing before
// anything downstream reasons about it.
func (s *Server) packs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.deps.Packs == nil {
		http.Error(w, "packs not configured", http.StatusNotImplemented)
		return
	}

	dryRun := r.URL.Query().Get("apply") != "true"

	stats, err := s.deps.Packs(context.WithoutCancel(r.Context()), dryRun)
	if err != nil {
		s.deps.Logger.Error("packs failed", "dry_run", dryRun, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.deps.Logger.Info("packs", "dry_run", dryRun, "stats", stats)
	writeJSON(w, s.deps.Logger, stats)
}

// tags projects managarr's view onto the client's tags.
//
// DRY RUN BY DEFAULT, as on /cleanup and /orphans. This one replaces every tag
// on every torrent, so the default matters even though the action is
// recoverable — the next pass corrects a wrong set, but only after it has been
// wrong in the client for an hour.
func (s *Server) tags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.deps.Tags == nil {
		http.Error(w, "tags not configured", http.StatusNotImplemented)
		return
	}

	dryRun := r.URL.Query().Get("apply") != "true"

	stats, err := s.deps.Tags(context.WithoutCancel(r.Context()), dryRun)
	if err != nil {
		s.deps.Logger.Error("tags failed", "dry_run", dryRun, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.deps.Logger.Info("tags", "dry_run", dryRun, "stats", stats)
	writeJSON(w, s.deps.Logger, stats)
}

// reap removes data and link trees nothing claims.
//
// DRY RUN BY DEFAULT, as on /cleanup and /orphans. Only the disk-side classes
// are ever deleted; the two inverse classes describe rows pointing at files
// that are already gone and have nothing to remove.
func (s *Server) reap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.deps.Reap == nil {
		http.Error(w, "reap not configured", http.StatusNotImplemented)
		return
	}

	dryRun := r.URL.Query().Get("apply") != "true"

	stats, err := s.deps.Reap(context.WithoutCancel(r.Context()), dryRun)
	if err != nil {
		s.deps.Logger.Error("reap failed", "dry_run", dryRun, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.deps.Logger.Info("reap", "dry_run", dryRun, "stats", stats)
	writeJSON(w, s.deps.Logger, stats)
}

// orphans deletes library entries nothing provides any more.
//
// DRY RUN BY DEFAULT, for the reasons on /cleanup and more so: this one removes
// media files. A dry run answers with the full candidate list rather than a
// count, because "five files would go" is not something anyone should approve
// without seeing which five.
func (s *Server) orphans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.deps.Orphans == nil {
		http.Error(w, "orphans not configured", http.StatusNotImplemented)
		return
	}

	// As on /cleanup: anything but an explicit apply=true is a dry run.
	dryRun := r.URL.Query().Get("apply") != "true"

	stats, err := s.deps.Orphans(context.WithoutCancel(r.Context()), dryRun)
	if err != nil {
		s.deps.Logger.Error("orphans failed", "dry_run", dryRun, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.deps.Logger.Info("orphans", "dry_run", dryRun, "stats", stats)
	writeJSON(w, s.deps.Logger, stats)
}

// cleanup removes torrents their tracker has unregistered.
//
// DRY RUN BY DEFAULT. ?apply=true is the only thing that deletes, and that
// asymmetry is deliberate: every other endpoint here either records something
// or triggers a read, and a timer misconfigured onto this one should report
// what it would have done rather than do it.
//
// Synchronous, unlike /reconcile. The work is a query and one API call, and a
// caller that has just deleted data should be told what went in the same
// response rather than having to go and read the log.
func (s *Server) cleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.deps.Cleanup == nil {
		http.Error(w, "cleanup not configured", http.StatusNotImplemented)
		return
	}

	// Anything other than an explicit apply=true is a dry run, including a
	// malformed value. "apply=maybe" must not delete torrents.
	dryRun := r.URL.Query().Get("apply") != "true"

	stats, err := s.deps.Cleanup(context.WithoutCancel(r.Context()), dryRun)
	if err != nil {
		s.deps.Logger.Error("cleanup failed", "dry_run", dryRun, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.deps.Logger.Info("cleanup", "dry_run", dryRun, "stats", stats)
	writeJSON(w, s.deps.Logger, stats)
}

// writeJSON sends a result. The status is already committed by the time
// encoding can fail, so there is nothing left to tell the client.
func writeJSON(w http.ResponseWriter, log *slog.Logger, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error("encode response failed", "err", err)
	}
}

// sidecarBatch is what a sidecar's relay posts.
type sidecarBatch struct {
	Events []sidecarEvent `json:"events"`
}

// sidecarEvent mirrors sidecar.Event over the fields this side acts on.
//
// Declared here rather than imported so the server does not depend on the
// sidecar package for a wire shape: the payload is passed through untouched and
// only the envelope is read.
type sidecarEvent struct {
	Source  string          `json:"source"`
	Kind    string          `json:"kind"`
	Ref     string          `json:"ref"`
	Payload json.RawMessage `json:"payload"`
}

// ingestSidecar records a relayed batch.
//
// Answers 202 on anything it can read at all, for the same reason every /hook/
// path answers 200: the sender is a sidecar that will log an error and move on,
// and the event is already gone by then. A body this cannot parse is a defect
// on one side or the other and retrying repeats it.
func (s *Server) ingestSidecar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		s.deps.Logger.Error("sidecar relay unreadable", "remote", remoteHost(r), "err", err)
		s.metrics.requests.WithLabelValues("sidecar", "unreadable").Inc()
		http.Error(w, "unreadable body", http.StatusBadRequest)
		return
	}

	var batch sidecarBatch
	if err := json.Unmarshal(body, &batch); err != nil {
		s.deps.Logger.Error("sidecar relay undecodable",
			"remote", remoteHost(r), "bytes", len(body),
			"body", redact.Text(string(body)), "err", err)
		s.metrics.requests.WithLabelValues("sidecar", "undecodable").Inc()
		w.WriteHeader(http.StatusAccepted)
		return
	}

	for _, e := range batch.Events {
		if e.Source == "" {
			// Unroutable. The envelope's whole job is to say who sent it, and
			// an event without that cannot reach a handler.
			s.deps.Logger.Warn("sidecar event has no source",
				"remote", remoteHost(r), "kind", e.Kind)
			s.metrics.requests.WithLabelValues("sidecar", "no_source").Inc()
			continue
		}
		s.metrics.requests.WithLabelValues(e.Source, "relayed").Inc()
		s.metrics.bodySize.WithLabelValues(e.Source).Observe(float64(len(e.Payload)))

		s.deps.Logger.Info("sidecar event",
			"source", e.Source, "kind", e.Kind, "ref", e.Ref,
			"payload", redact.Text(string(e.Payload)))

		if s.deps.Ingest != nil && len(e.Payload) > 0 {
			ictx, cancel := context.WithTimeout(
				context.WithoutCancel(r.Context()), ingestTimeout)
			s.deps.Ingest(ictx, e.Source, e.Payload)
			cancel()
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

// reconcile runs a full pass. Driven by a systemd timer rather than a ticker in
// this process: a pass stats every file of every torrent over NFS and has
// nothing to do with answering a webhook inside a request timeout.
//
// Answers 202 and runs to completion in the background, so the timer unit is
// not held open for the duration. An overlapping tick gets 409 and is a no-op.
func (s *Server) reconcile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.deps.Reconcile == nil {
		http.Error(w, "reconcile not configured", http.StatusNotImplemented)
		return
	}
	if !s.running.CompareAndSwap(false, true) {
		http.Error(w, "already running", http.StatusConflict)
		return
	}

	go func() {
		defer s.running.Store(false)
		// Detached from the request: the caller has already been
		// answered, and cancelling on its disconnect would abort a pass
		// halfway through.
		if err := s.deps.Reconcile(context.WithoutCancel(r.Context())); err != nil {
			s.deps.Logger.Error("reconcile failed", "err", err)
		}
	}()

	w.WriteHeader(http.StatusAccepted)
	_, _ = io.WriteString(w, "started\n")
}

// record logs one request in full and answers 200.
func (s *Server) record(source string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		started := s.deps.Now()

		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
		if err != nil {
			// Still a 200: a truncated body is a recorded body, and a
			// retry storm would only add noise to the corpus.
			s.deps.Logger.Error("hook body read failed",
				"source", source, "err", err, "remote", remoteHost(r))
			s.metrics.requests.WithLabelValues(source, "read_error").Inc()
			w.WriteHeader(http.StatusOK)
			return
		}

		s.metrics.requests.WithLabelValues(source, "ok").Inc()
		s.metrics.bodySize.WithLabelValues(source).Observe(float64(len(body)))

		// Redacted before it is logged, because this log ships to Loki.
		//
		// autobrr's action posts {{ .TorrentUrl }} on every grab, which for a
		// private tracker is the account's passkey; cross-seed's own webhook
		// URL carries an apikey in the query. Logging those verbatim put live
		// credentials into the log store continuously — and it kept doing so
		// after a migration had scrubbed the same values out of Postgres,
		// because the redaction lived in a package this one could not reach.
		//
		// Headers are allowlisted rather than redacted: the useful ones are a
		// known, short set, and every remaining one is either noise or a place
		// a sender might put a bearer token. Names of the dropped headers are
		// kept so a missing field is visible without exposing its value.
		s.deps.Logger.Info("hook",
			"source", source,
			"method", r.Method,
			"path", r.URL.Path,
			"query", redact.URL(r.URL.RawQuery),
			"remote", remoteHost(r),
			"content_type", r.Header.Get("Content-Type"),
			"content_length", r.ContentLength,
			"body_bytes", len(body),
			"headers", safeHeaders(r.Header),
			"body", redact.Text(string(body)),
			"received_at", started.UTC().Format(time.RFC3339Nano),
		)

		// Synchronous, before the 200. Ordering matters — an "added" and a
		// "deleted" for one torrent arrive seconds apart and must be applied
		// in that order — and doing the writes inline is also what makes a
		// slow database show up as sender back-pressure rather than as an
		// unbounded pile of goroutines.
		//
		// Detached from the request context with its own deadline: a sender
		// that times out or disconnects would otherwise cancel the writes
		// halfway through a batch, and Vector batches up to two dozen objects
		// into one delivery. The redelivery would then be applied on top of a
		// half-written state.
		if s.deps.Ingest != nil {
			ctx, cancel := context.WithTimeout(
				context.WithoutCancel(r.Context()), ingestTimeout)
			s.deps.Ingest(ctx, source, body)
			cancel()
		}

		w.WriteHeader(http.StatusOK)
	}
}

// loggedHeaders are the request headers whose VALUES may be logged.
//
// An allowlist rather than a denylist: a denylist has to anticipate every name
// a sender might choose for its credential, and senders here include two that
// take the header name from user configuration.
var loggedHeaders = map[string]bool{
	"Content-Type":   true,
	"Content-Length": true,
	"User-Agent":     true,
	"Accept":         true,
}

// safeHeaders renders headers as a single logfmt-safe string.
//
// Allowlisted names are rendered with their values; every other name is
// rendered alone, so a sender adding a header is still visible without its
// value being captured. A map would serialise better, but Vector's JSON
// encoder flattens nested maps into dotted keys and header names are not
// stable enough to be worth that cardinality in Loki.
func safeHeaders(h http.Header) string {
	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}
	// Sorted so two requests with the same headers produce the same line and
	// Loki can deduplicate them.
	sort.Strings(names)

	var b strings.Builder
	for i, name := range names {
		if i > 0 {
			b.WriteString("; ")
		}
		if loggedHeaders[name] {
			fmt.Fprintf(&b, "%s=%s", name, strings.Join(h[name], ","))
			continue
		}
		// Name only. The value may be a bearer token.
		b.WriteString(name)
	}
	return b.String()
}

// remoteHost strips the ephemeral port so the value groups in Loki.
func remoteHost(r *http.Request) string {
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return addr[:i]
	}
	return addr
}

// Config is the server's tunable surface.
type Config struct {
	// Tags are `json`, not `koanf`: homelab/config unmarshals with the json
	// tag unless a tool overrides it, and app does not. A koanf tag here is
	// silently ignored — every field keeps its zero value and the flags
	// appear to do nothing.
	Addr            string        `json:"addr"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	DBDSN           string        `json:"db_dsn"`
	QBittorrentURL  string        `json:"qbittorrent_url"`
	SonarrURL       string        `json:"sonarr_url"`
	RadarrURL       string        `json:"radarr_url"`
	LidarrURL       string        `json:"lidarr_url"`
	BazarrURL       string        `json:"bazarr_url"`
	SlskdURL        string        `json:"slskd_url"`
	AutobrrURL      string        `json:"autobrr_url"`

	// The managarr-* sidecars, one per *arr. Each serves the same history its
	// service does, with the since-cursor the service itself does not offer —
	// without one, every hourly pass replays all 28,000 records to find the
	// handful that are new.
	//
	// Empty falls back to reading /history directly. That is a real fallback
	// rather than a degraded mode: it is correct, just wasteful, so a sidecar
	// being down costs the cursor and not the pass.
	SonarrSidecarURL string `json:"sonarr_sidecar_url"`
	RadarrSidecarURL string `json:"radarr_sidecar_url"`
	LidarrSidecarURL string `json:"lidarr_sidecar_url"`

	// The remaining sidecars. Bazarr has none by design: its pass decorates a
	// full subtitle snapshot with history on every run, so a cursor would
	// leave every subtitle first seen after it without a provider or score.
	AutobrrSidecarURL  string `json:"autobrr_sidecar_url"`
	ProwlarrSidecarURL string `json:"prowlarr_sidecar_url"`
	SlskdSidecarURL    string `json:"slskd_sidecar_url"`
	BeetsSidecarURL    string `json:"beets_sidecar_url"`

	// StatsInterval is how often the domain gauges are recomputed.
	//
	// Independent of the reconcile, which is hourly because it stats every
	// file over NFS. These are indexed aggregates costing milliseconds, so
	// pinning them to the pass would discard every trend finer than an hour.
	StatsInterval time.Duration `json:"stats_interval"`
	// AutobrrFull walks the whole release history instead of stopping at the
	// first page with nothing new. Needed once, to backfill; a scheduled pass
	// leaves it off.
	AutobrrFull bool   `json:"autobrr_full"`
	ProwlarrURL string `json:"prowlarr_url"`
	// ProwlarrFull walks Prowlarr's whole history rather than stopping at the
	// first page holding nothing new. Needed once, to backfill.
	ProwlarrFull bool `json:"prowlarr_full"`
	// CrossSeedURL is the base URL of the crossseed-api sidecar, which serves
	// cross-seed.s decision log. cross-seed.s own API has no decisions route
	// and its webhook fires only on a match, so the rejections — most of the
	// record — are reachable no other way. Empty skips the replay.
	CrossSeedURL string `json:"cross_seed_url"`
	// BeetsURL is the base URL of beets' web plugin. Empty skips the tag pass.
	//
	// The plugin's include_paths option must be enabled: it is off by default,
	// and without it every item comes back with a null path, which is the only
	// field tying a tagged item to a file.
	BeetsURL string `json:"beets_url"`
	// RipsDir is the CD-rip staging root, laid out as <artist>/<album>/.
	// Empty skips the scan.
	RipsDir string `json:"rips_dir"`

	// CompleteDir is the download tree, laid out as <category>/<entry>. Empty
	// disables the reap sweep's download-data check.
	CompleteDir string `json:"complete_dir"`
	// LinksDir is cross-seed's link tree, laid out as <tracker>/<entry>.
	LinksDir string `json:"links_dir"`
	// IncompleteDir is where partial downloads live. Usually empty here: the
	// directory belongs to the torrent client's container and is not normally
	// mounted into this one.
	IncompleteDir string `json:"incomplete_dir"`
	// LibraryDirs are the trees the *arrs import into, used to bound the
	// missing-file check so a row pointing somewhere unmounted is skipped
	// rather than reported as missing.
	LibraryDirs []string `json:"library_dirs"`
	// PacksDir is seasonpackarr's preImportPath. Content under it is a season
	// pack assembled locally from episodes already held, which is a third origin
	// alongside a grab and a cross-seed inject.
	PacksDir string `json:"packs_dir"`
	// SeedRequirementsJSON is what each tracker demands before a torrent may
	// stop, keyed by announce host, as JSON:
	//
	//	{"tracker.torrentleech.org":{"days":10,"ratio":1.0,"timeNeedsComplete":true}}
	//
	// JSON rather than repeated key=value flags because the value is a record,
	// and flattening it into a bespoke string encoding would need its own
	// parser and its own failure modes.
	//
	// A host with no entry yields seed::unknown rather than an assumption.
	SeedRequirementsJSON string `json:"seed_requirements"`
	// PreserveTags are tags another tool owns and reads back, kept on any
	// torrent already carrying one instead of being replaced.
	//
	// cross-seed is the case: its preFilter tests for its own tag to decide
	// what NOT to search from, and 6.13.7 has no setting that disables
	// tagging.
	PreserveTags []string `json:"preserve_tags"`
	// ReapCategories limits the download tree to what torrents actually manage.
	//
	// The same tree holds soulseek/, rips/ and youtube/, which no torrent ever
	// claimed — "unclaimed" says nothing about those, and treating them as
	// leftovers would propose deleting the entire Soulseek library.
	ReapCategories []string `json:"reap_categories"`

	// TrackerTags maps an announce host to the value its tracker:: tag carries.
	//
	// Declared rather than derived: one tracker announces under several
	// hostnames whose domains share no common part — tracker.torrentleech.org
	// and tracker.tleechreload.org are the same site — so without the map a
	// torrent carries two tracker tags for one tracker. An unmapped host tags
	// with its own hostname, which is visible rather than silent.
	//
	// Given on the command line as host=VALUE pairs.
	TrackerTags map[string]string `json:"tracker_tags"`
}

// Defaults is what the flags are populated from.
func Defaults() Config {
	return Config{
		Addr:            ":9911",
		ShutdownTimeout: 10 * time.Second,
		DBDSN:           "postgres://managarr@127.0.0.1:5432/managarr?sslmode=disable",
		// One minute. These are indexed aggregates over tables that change
		// continuously; finer would cost more than the resolution is worth,
		// coarser would blur the funnel rates the dashboard is for.
		StatsInterval: time.Minute,
	}
}

// Serve runs the listener until ctx is cancelled, then drains.
func Serve(ctx context.Context, cfg *Config, h http.Handler, logger *slog.Logger) error {
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", cfg.Addr, "sources", strings.Join(Sources, ","))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	}
}
