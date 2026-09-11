// Package server is stashtrap's HTTP surface: the endpoint autobrr asks about
// each release, plus liveness and readiness.
//
// autobrr calls this as an EXTERNAL FILTER, not as an action. It asks "should I
// take this?" and reads the answer from the status code, then performs the
// grab itself with its own client, retries and release history. That split is
// deliberate: stashtrap owns the decision, autobrr owns the mechanics, and a
// stashtrap outage stops grabs rather than corrupting them.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"homelab/stashtrap/internal/parse"
	"homelab/stashtrap/internal/policy"
	"homelab/stashtrap/internal/store"
	"homelab/stashtrap/internal/sync"
	"homelab/stashtrap/internal/torrent"
	"homelab/stashtrap/internal/types"
)

// Status codes for the filter answer.
//
// autobrr compares the response status against the filter's
// webhook_expect_status: match means the release passes the external check.
// 250 is seasonpackarr's convention in this homelab and is kept for
// consistency — it is not an HTTP status anyone else will confuse for a
// success, which is the point.
const (
	// StatusAccept is what autobrr's expect_status must be set to.
	StatusAccept = 250
	// StatusReject is any other 2xx: handled, understood, not wanted.
	StatusReject = http.StatusOK
)

// maxBodyBytes bounds a request. A .torrent with a large file table can run to
// a few hundred kilobytes once base64'd; 16 MiB is far above anything real and
// still stops a malformed template from exhausting memory.
const maxBodyBytes = 16 << 20

// Deps is what the server needs. Injected rather than constructed so tests can
// supply a temporary store and read the metrics back.
type Deps struct {
	Store   *store.Store
	Decider *policy.Decider
	Logger  *slog.Logger
	// Syncer fills the stash-box catalogue. Nil disables POST /sync, which
	// is what a deployment without an API key gets.
	Syncer *sync.Syncer
	// Endpoint namespaces catalogue lookups. Empty disables matching, which
	// is the same deployment that has no catalogue to match against.
	Endpoint string
	// Now is the clock, injectable so a test can assert on stored times.
	Now func() time.Time
}

// Server serves the endpoints.
type Server struct {
	deps    Deps
	metrics *metrics
}

// metrics are per-server rather than global so two servers in one test binary
// do not fight over a registry.
type metrics struct {
	announces  *prometheus.CounterVec
	categories *prometheus.CounterVec
	sizes      prometheus.Histogram
	decide     prometheus.Histogram
	files      prometheus.Histogram
	// What the matcher made of each name, which the decision rests on and
	// which the announce counters cannot show: a release can be recognised
	// in several ways and still name nobody followed.
	recognised *prometheus.CounterVec
	favourites *prometheus.CounterVec
	scenes     *prometheus.CounterVec
	score      prometheus.Histogram
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		announces: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "stashtrap_announces_total",
			Help: "Announces received, by indexer and outcome.",
		}, []string{"indexer", "decision", "reason"}),
		// A separate counter rather than a label on announces_total: an
		// announce carries more than one category, so folding them in
		// would multiply every other series by the category count and make
		// a plain announce total wrong.
		//
		// Both the id and the name are labels. The id is the stable key —
		// the name comes from the indexer's caps and can change, which on
		// BitPorn it eventually should, since Prowlarr currently misspells
		// every site-specific name with a trailing double quote. Expect one
		// series to be replaced by another when that is fixed.
		categories: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "stashtrap_announce_categories_total",
			Help: "Category assignments seen on announces, by category.",
		}, []string{"category_id", "category"}),
		sizes: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "stashtrap_announce_size_bytes",
			Help: "Announced release size.",
			// 64 MiB to 64 GiB: scene releases cluster in the low
			// gigabytes and packs run to tens.
			Buckets: prometheus.ExponentialBuckets(64<<20, 2, 11),
		}),
		decide: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "stashtrap_decide_seconds",
			Help:    "Time to parse, decide and record one announce.",
			Buckets: prometheus.DefBuckets,
		}),
		files: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "stashtrap_announce_files",
			Help: "Files per announced torrent, where the torrent was sent.",
			// A pack is the interesting case: 1 file is a scene, 40 is
			// a compilation that maps to 40 upstream scenes.
			Buckets: []float64{1, 2, 5, 10, 25, 50, 100, 250},
		}),
		recognised: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "stashtrap_recognised_total",
			Help: "Entities recognised in release names, by kind.",
		}, []string{"kind"}),
		// Labelled by name rather than id: a dashboard showing which of the
		// followed performers actually appear is only readable if it says
		// who. Bounded by the size of the follow list, which is the point of
		// the follow list.
		favourites: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "stashtrap_favourite_hits_total",
			Help: "Releases naming a followed entity, by kind and name.",
		}, []string{"kind", "name"}),
		scenes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "stashtrap_scenes_identified_total",
			Help: "Releases identified as a known scene, by whether the parts agreed.",
		}, []string{"corroborated"}),
		score: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "stashtrap_reading_score",
			Help: "Evidence behind each reading; zero means nothing was recognised.",
			// The weights are single digits and a corroborated reading
			// stacks a few, so anything past 20 is off the scale.
			Buckets: []float64{0, 1, 2.5, 4, 6, 9, 12, 16, 20},
		}),
	}
	reg.MustRegister(
		m.announces, m.categories, m.sizes, m.decide, m.files,
		m.recognised, m.favourites, m.scenes, m.score,
	)
	return m
}

// New builds a Server and registers its metrics.
func New(deps Deps, reg prometheus.Registerer) (*Server, error) {
	switch {
	case deps.Store == nil:
		return nil, errors.New("server: Store is required")
	case deps.Decider == nil:
		return nil, errors.New("server: Decider is required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &Server{deps: deps, metrics: newMetrics(reg)}, nil
}

// Handler returns the mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /announce", s.handleAnnounce)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /readyz", s.handleReady)
	if s.deps.Syncer != nil {
		mux.HandleFunc("POST /sync", s.handleSync)
	}
	return mux
}

// handleSync fills the catalogue from the stash-box.
//
// Triggered by a systemd timer rather than run on a ticker inside the process:
// a first fill takes minutes and has nothing to do with answering autobrr,
// which happens inside a request timeout.
//
// The crawl outlives the request. A timer's curl gets 202 and goes away, while
// the work continues against a context tied to the server's lifetime, not the
// client's — otherwise the timer's own timeout would abort the fill halfway.
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if s.deps.Syncer.Running() {
		http.Error(w, "sync already in progress", http.StatusConflict)
		return
	}

	go func() {
		ctx := context.WithoutCancel(r.Context())
		if _, err := s.deps.Syncer.Run(ctx); err != nil && !errors.Is(err, sync.ErrInProgress) {
			s.deps.Logger.Error("sync failed", "error", err)
		}
	}()

	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintln(w, "sync started") //nolint:errcheck // the timer is not reading this
}

// handleAnnounce records one release and answers autobrr.
//
// A malformed body is a 400 and is not recorded: it is a bug in the filter
// template, and recording it as an announce would poison the corpus with rows
// that never described a release.
func (s *Server) handleAnnounce(w http.ResponseWriter, r *http.Request) {
	start := s.deps.Now()
	defer func() {
		s.metrics.decide.Observe(s.deps.Now().Sub(start).Seconds())
	}()

	var a types.Announce
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err := dec.Decode(&a); err != nil {
		s.deps.Logger.Warn("malformed announce", "error", err)
		http.Error(w, "malformed announce", http.StatusBadRequest)
		return
	}

	meta, torrentErr := parseTorrent(a)
	if torrentErr != "" {
		// Not fatal: the announce is still worth recording, and a
		// tracker that changes its .torrent output should show up as a
		// column full of errors rather than as silence.
		s.deps.Logger.Warn("torrent parse failed", "announce", a, "error", torrentErr)
	}
	if meta != nil {
		s.metrics.files.Observe(float64(len(meta.Files)))
	}

	reading := s.readName(r.Context(), a)
	match := s.favourites(r.Context(), reading)
	decision := s.deps.Decider.Decide(a, meta, match)

	id, err := s.deps.Store.Insert(r.Context(), store.Record{
		ReceivedAt:   start,
		Announce:     a,
		Torrent:      meta,
		TorrentError: torrentErr,
		Accepted:     decision.Accept,
		Reason:       decision.Reason,
		Reading:      recordReading(reading, match),
	})
	if err != nil {
		// Refuse rather than answer from memory: an accept that was not
		// recorded is a grab nothing can later reconcile.
		s.deps.Logger.Error("record announce", "announce", a, "error", err)
		http.Error(w, "record failed", http.StatusInternalServerError)
		return
	}

	outcome := "reject"
	if decision.Accept {
		outcome = "accept"
	}
	s.metrics.announces.WithLabelValues(a.Indexer, outcome, decision.Reason).Inc()
	s.observeReading(r.Context(), reading, match)
	for _, c := range types.ParseCategories(a.Categories) {
		s.metrics.categories.WithLabelValues(strconv.Itoa(c.ID), c.Name).Inc()
	}
	s.metrics.sizes.Observe(float64(a.Size))
	// Matches are attached only when there are some: slog renders an empty
	// slice as `matches=[]`, which reads like a lookup that failed rather
	// than a release nothing in the catalogue is named in.
	args := []any{
		"id", id,
		"announce", a,
		"decision", outcome,
		"reason", decision.Reason,
		"files", fileCount(meta),
		// Name-derived, and zero on the third of releases that claim
		// nothing. Logged rather than acted on: what to do with the
		// silent ones is the open question the logs are meant to answer.
		"resolution", types.ParseResolution(a.Name),
	}
	if reading != nil {
		args = append(args, readingLog(*reading)...)
	}
	// The favourites are the decision, so they are logged whenever there are
	// any — an accept has to be explicable from its own log line.
	if len(match.Performers) > 0 {
		args = append(args, "favourite_performers", match.Performers)
	}
	if len(match.Studios) > 0 {
		args = append(args, "favourite_studios", match.Studios)
	}
	s.deps.Logger.Info("announce", args...)

	if decision.Accept {
		w.WriteHeader(StatusAccept)
	} else {
		w.WriteHeader(StatusReject)
	}
}

// readName parses the release name and resolves the reading the catalogue
// agrees with most, or nil when no reading resolved to anything.
//
// A failure is logged and swallowed: this is an aid to a decision that
// currently rejects everything, and a query error must not turn into a 500 that
// autobrr reads as the gate being down.
func (s *Server) readName(ctx context.Context, a types.Announce) *parse.Result {
	if s.deps.Endpoint == "" {
		return nil
	}
	res, ok, err := parse.Identify(ctx, s.deps.Store.NewCatalogue(s.deps.Endpoint), store.Normalise, a.Name)
	if err != nil {
		s.deps.Logger.Error("read announce name", "announce", a, "error", err)
		return nil
	}
	if !ok {
		return nil
	}
	return &res
}

// observeReading records what the matcher made of one name.
//
// Separate from the announce counter because the two answer different
// questions: that one says what was decided, this says what was recognised,
// and a release can be recognised several ways and still name nobody followed.
func (s *Server) observeReading(ctx context.Context, reading *parse.Result, match policy.Match) {
	if reading == nil {
		s.metrics.score.Observe(0)
		return
	}
	s.metrics.score.Observe(reading.Score)

	seen := map[string]bool{}
	for _, m := range reading.Matches {
		if key := m.Kind + m.ID; !seen[key] {
			seen[key] = true
			s.metrics.recognised.WithLabelValues(m.Kind).Inc()
		}
	}
	if reading.SceneID != "" {
		s.metrics.scenes.WithLabelValues(strconv.FormatBool(reading.Corroborated)).Inc()
	}

	if len(match.Performers) == 0 && len(match.Studios) == 0 {
		return
	}
	names, err := s.deps.Store.EntityNames(ctx, s.deps.Endpoint, match.Performers, match.Studios)
	if err != nil {
		s.deps.Logger.Error("entity names", "error", err)
		return
	}
	for kind, ids := range map[string][]string{
		parse.KindPerformer: match.Performers,
		parse.KindStudio:    match.Studios,
	} {
		for _, id := range ids {
			name := names[id]
			if name == "" {
				name = id
			}
			s.metrics.favourites.WithLabelValues(kind, name).Inc()
		}
	}
}

// recordReading turns a reading into the form the store keeps, marking which
// of the entities were followed.
//
// The followed flag is stored rather than joined on later: whether an entity
// was followed at the moment of the decision is what explains the decision, and
// a list edited afterwards would rewrite history.
func recordReading(reading *parse.Result, match policy.Match) store.Reading {
	if reading == nil {
		return store.Reading{}
	}

	favourite := make(map[string]bool, len(match.Performers)+len(match.Studios))
	for _, id := range match.Performers {
		favourite[id] = true
	}
	for _, id := range match.Studios {
		favourite[id] = true
	}

	out := store.Reading{
		SceneID:      reading.SceneID,
		Score:        reading.Score,
		Corroborated: reading.Corroborated,
	}
	for _, m := range reading.Matches {
		out.Matches = append(out.Matches, store.RecordedMatch{
			Kind:      m.Kind,
			EntityID:  m.ID,
			Probe:     m.Probe,
			Favourite: favourite[m.ID],
		})
	}
	// Performers the release did not name but the identified scene credits
	// are part of the answer too, and the probe records that they came from
	// the scene rather than from the text.
	for _, id := range reading.PerformerIDs {
		if !slices.ContainsFunc(out.Matches, func(m store.RecordedMatch) bool {
			return m.Kind == parse.KindPerformer && m.EntityID == id
		}) {
			out.Matches = append(out.Matches, store.RecordedMatch{
				Kind:      parse.KindPerformer,
				EntityID:  id,
				Probe:     scenePerformerProbe,
				Favourite: favourite[id],
			})
		}
	}
	if reading.SceneID != "" {
		out.Matches = append(out.Matches, store.RecordedMatch{
			Kind:     parse.KindScene,
			EntityID: reading.SceneID,
			Probe:    reading.Body,
		})
	}
	return out
}

// scenePerformerProbe marks a performer taken from the identified scene rather
// than found in the release name.
const scenePerformerProbe = "(scene cast)"

// favourites narrows a reading to the followed entities it named, which is
// what the decision turns on.
//
// A failure is reported as an empty catalogue rather than as no favourites:
// rejecting because nothing was found must not be confused with rejecting
// because the question could not be asked.
func (s *Server) favourites(ctx context.Context, reading *parse.Result) policy.Match {
	if s.deps.Endpoint == "" {
		return policy.Match{CatalogueEmpty: true}
	}
	performers, studios, scenes, err := s.deps.Store.CatalogueCounts(ctx, s.deps.Endpoint)
	if err != nil {
		s.deps.Logger.Error("catalogue counts", "error", err)
		return policy.Match{CatalogueEmpty: true}
	}
	if performers+studios+scenes == 0 {
		return policy.Match{CatalogueEmpty: true}
	}
	if reading == nil {
		return policy.Match{}
	}

	favPerformers, favStudios, err := s.deps.Store.Favourites(
		ctx, s.deps.Endpoint, reading.PerformerIDs, reading.StudioIDs)
	if err != nil {
		s.deps.Logger.Error("resolve favourites", "error", err)
		return policy.Match{CatalogueEmpty: true}
	}
	return policy.Match{Performers: favPerformers, Studios: favStudios}
}

// readingLog renders a reading for one log record.
//
// The groups the parser split out are logged beside the ids they resolved to,
// because a wrong answer is only obvious next to the text it came from: "studio
// %q -> 1" says more than an id on its own ever will.
func readingLog(r parse.Result) []any {
	args := []any{"score", r.Score, "body", r.Body, "corroborated", r.Corroborated}
	if r.SceneID != "" {
		args = append(args, "scene_id", r.SceneID)
	}
	if names := r.Names(); len(names) > 0 {
		args = append(args, "names", names)
	}
	if len(r.PerformerIDs) > 0 {
		args = append(args, "performer_ids", r.PerformerIDs)
	}
	if len(r.StudioIDs) > 0 {
		args = append(args, "studio_ids", r.StudioIDs)
	}
	if r.Date != "" {
		args = append(args, "date", r.Date)
	}
	return args
}

// parseTorrent decodes the attached .torrent, returning the metadata or a
// human-readable reason it could not be read.
func parseTorrent(a types.Announce) (meta *torrent.Meta, parseErr string) {
	raw, err := a.Torrent()
	if err != nil {
		return nil, err.Error()
	}
	if raw == nil {
		return nil, ""
	}
	meta, err = torrent.Parse(raw)
	if err != nil {
		return nil, err.Error()
	}
	return meta, ""
}

func fileCount(m *torrent.Meta) int {
	if m == nil {
		return 0
	}
	return len(m.Files)
}

// handleHealth reports that the process is up.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok") //nolint:errcheck // a failed health write has nobody to tell
}

// handleReady reports that the database is usable, which is the only
// dependency phase one has.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if _, err := s.deps.Store.Count(r.Context()); err != nil {
		s.deps.Logger.Error("readiness", "error", err)
		http.Error(w, "store unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ready") //nolint:errcheck // as above
}
