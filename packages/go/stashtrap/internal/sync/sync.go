// Package sync fills the local catalogue from a stash-box.
//
// Matching is constrained to what the account follows, so the catalogue is the
// matcher's whole world: favourited performers, and every scene attached to
// them or to a favourited studio. A release naming someone unfollowed is not a
// miss, it is out of scope.
//
// Studios are the exception and are mirrored whole. They are not a watch list
// but a vocabulary: the parser has to decide whether a release's leading
// bracket names a site or is part of the title, and only the full list answers
// that.
//
// The crawl is driven by a timer rather than a loop inside the process: a full
// fill runs for minutes and has nothing to do with answering autobrr, which
// has to happen inside a request timeout.
package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"homelab/clients/stashbox"
	"homelab/stashtrap/internal/store"
)

// Entities recorded in sync_state, one row each.
const (
	entityPerformers = "performers"
	entityStudios    = "studios"
	entityScenes     = "scenes"
)

// ErrInProgress reports a sync that was asked to start while one was running.
var ErrInProgress = errors.New("sync: already in progress")

// Syncer fills the catalogue.
type Syncer struct {
	client   *stashbox.Client
	store    *store.Store
	logger   *slog.Logger
	metrics  *metrics
	endpoint string
	perPage  int

	// running is the single-flight guard. A timer that fires while a long
	// first fill is still going must not start a second crawl against the
	// same tables and the same rate limit.
	mu      sync.Mutex
	running bool
}

// Deps is what a Syncer needs.
type Deps struct {
	Client *stashbox.Client
	Store  *store.Store
	Logger *slog.Logger
	// Endpoint namespaces every row, since ids from two stash-boxes share
	// a format but not a meaning.
	Endpoint string
	// PerPage is the page size; zero uses the client default.
	PerPage int
	// Registry receives the catalogue and crawl metrics. Nil disables them,
	// which is what a test that does not assert on them wants.
	Registry prometheus.Registerer
}

// New builds a Syncer.
func New(d Deps) (*Syncer, error) {
	switch {
	case d.Client == nil:
		return nil, errors.New("sync: client is required")
	case d.Store == nil:
		return nil, errors.New("sync: store is required")
	case d.Endpoint == "":
		return nil, errors.New("sync: endpoint is required")
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.PerPage == 0 {
		d.PerPage = stashbox.DefaultPerPage
	}
	s := &Syncer{
		client:   d.Client,
		store:    d.Store,
		logger:   d.Logger,
		endpoint: d.Endpoint,
		perPage:  d.PerPage,
	}
	if d.Registry != nil {
		s.metrics = newMetrics(d.Registry)
		// Publish what is already stored, so a restart does not read as an
		// empty catalogue until the next crawl.
		s.refreshCatalogue(context.Background())
	}
	return s, nil
}

// Result summarises one crawl.
type Result struct {
	Performers int    `json:"performers"`
	Studios    int    `json:"studios"`
	Scenes     int    `json:"scenes"`
	Duration   string `json:"duration"`
}

// Run performs a full crawl: favourites first, then their scenes.
//
// Order matters. Scenes are fetched per followed entity, so the favourites
// have to be known before there is anything to fetch scenes for.
func (s *Syncer) Run(ctx context.Context) (Result, error) {
	if !s.acquire() {
		return Result{}, ErrInProgress
	}
	defer s.release()

	start := time.Now()
	var res Result

	performers, err := s.syncPerformers(ctx)
	res.Performers = performers
	if err != nil {
		s.failed()
		return res, err
	}

	studios, err := s.syncStudios(ctx)
	res.Studios = studios
	if err != nil {
		s.failed()
		return res, err
	}

	scenes, err := s.syncScenes(ctx)
	res.Scenes = scenes
	elapsed := time.Since(start)
	res.Duration = elapsed.String()
	if err != nil {
		s.failed()
		return res, err
	}

	// A crawl reintroduces the derived aliases that make every label in a
	// network answer to the network's name, so they are pruned before the
	// frequencies that would otherwise count them.
	if err := s.store.PruneDerivedAliases(ctx, s.endpoint); err != nil {
		s.failed()
		return res, fmt.Errorf("sync prune aliases: %w", err)
	}

	// Alias frequencies are derived from what was just crawled, and matching
	// weights every hit by them, so they are rebuilt here rather than
	// recomputed on every announce.
	if err := s.store.RebuildAliasFreq(ctx, s.endpoint); err != nil {
		s.failed()
		return res, fmt.Errorf("sync alias frequencies: %w", err)
	}

	if s.metrics != nil {
		s.metrics.duration.Set(elapsed.Seconds())
		s.metrics.lastSuccess.SetToCurrentTime()
	}
	s.refreshCatalogue(ctx)

	s.logger.Info("sync complete",
		"performers", res.Performers,
		"studios", res.Studios,
		"scenes", res.Scenes,
		"duration", res.Duration,
	)
	return res, nil
}

// Running reports whether a crawl is in flight.
func (s *Syncer) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Syncer) acquire() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return false
	}
	s.running = true
	if s.metrics != nil {
		s.metrics.running.Set(1)
	}
	return true
}

func (s *Syncer) release() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
	if s.metrics != nil {
		s.metrics.running.Set(0)
	}
}

// failed records a crawl that ended in an error.
func (s *Syncer) failed() {
	if s.metrics != nil {
		s.metrics.failures.Inc()
	}
}

// fetched records items pulled from the stash-box, which is more than the rows
// stored whenever a scene arrives through both a performer and a studio.
func (s *Syncer) fetched(entity string, n int) {
	if s.metrics != nil && n > 0 {
		s.metrics.fetched.WithLabelValues(entity).Add(float64(n))
	}
}

// syncPerformers pages through the account's favourited performers.
func (s *Syncer) syncPerformers(ctx context.Context) (int, error) {
	return s.paged(ctx, entityPerformers, func(page int) (int, bool, error) {
		res, err := s.client.FavoritePerformers(ctx, page, s.perPage)
		if err != nil {
			return 0, false, err
		}
		for _, p := range res.Items {
			if err := s.store.UpsertPerformer(ctx, s.endpoint, performerFrom(p)); err != nil {
				return 0, false, err
			}
		}
		return len(res.Items), res.HasMore(page, s.perPage), nil
	})
}

// syncStudios mirrors every studio the stash-box knows.
//
// Not just the favourites, unlike performers. A release leads with the site
// that published it regardless of who is followed, so a studio segment can only
// be told apart from a title by looking it up — and a catalogue holding eight
// studios answers "not a studio" to almost everything. The favourite flag comes
// back on each row, so the crawl that mirrors also keeps FavouriteStudioIDs
// correct.
func (s *Syncer) syncStudios(ctx context.Context) (int, error) {
	return s.paged(ctx, entityStudios, func(page int) (int, bool, error) {
		res, err := s.client.AllStudios(ctx, page, s.perPage)
		if err != nil {
			return 0, false, err
		}
		for _, st := range res.Items {
			if err := s.store.UpsertStudio(ctx, s.endpoint, studioFrom(st)); err != nil {
				return 0, false, err
			}
		}
		return len(res.Items), res.HasMore(page, s.perPage), nil
	})
}

// syncScenes walks every followed performer and studio, storing their scenes.
//
// A scene credited to a followed performer at a followed studio arrives twice;
// the upsert is idempotent, so the duplicate costs a write rather than a row.
func (s *Syncer) syncScenes(ctx context.Context) (int, error) {
	if err := s.store.StartSync(ctx, s.endpoint, entityScenes); err != nil {
		return 0, err
	}

	performerIDs, err := s.store.FavouritePerformerIDs(ctx, s.endpoint)
	if err != nil {
		return 0, s.finish(ctx, entityScenes, 0, err)
	}
	studioIDs, err := s.store.FavouriteStudioIDs(ctx, s.endpoint)
	if err != nil {
		return 0, s.finish(ctx, entityScenes, 0, err)
	}

	total := 0
	for _, id := range performerIDs {
		n, err := s.scenesFor(ctx, id, s.client.ScenesForPerformer)
		total += n
		if err != nil {
			return total, s.finish(ctx, entityScenes, total, err)
		}
	}
	for _, id := range studioIDs {
		n, err := s.scenesFor(ctx, id, s.client.ScenesForStudio)
		total += n
		if err != nil {
			return total, s.finish(ctx, entityScenes, total, err)
		}
	}
	return total, s.finish(ctx, entityScenes, total, nil)
}

// scenesFor pages one entity's scenes through fetch.
func (s *Syncer) scenesFor(
	ctx context.Context,
	id string,
	fetch func(context.Context, string, int, int) (stashbox.Page[stashbox.Scene], error),
) (int, error) {
	total := 0
	for page := 1; ; page++ {
		res, err := fetch(ctx, id, page, s.perPage)
		if err != nil {
			return total, err
		}
		for _, sc := range res.Items {
			if err := s.store.UpsertScene(ctx, s.endpoint, sceneFrom(sc)); err != nil {
				return total, err
			}
		}
		total += len(res.Items)
		s.fetched(entityScenes, len(res.Items))
		if !res.HasMore(page, s.perPage) {
			return total, nil
		}
	}
}

// paged runs a paginated crawl, recording its start and outcome.
func (s *Syncer) paged(ctx context.Context, entity string, fetch func(page int) (int, bool, error)) (int, error) {
	if err := s.store.StartSync(ctx, s.endpoint, entity); err != nil {
		return 0, err
	}
	total := 0
	for page := 1; ; page++ {
		n, more, err := fetch(page)
		total += n
		s.fetched(entity, n)
		if err != nil {
			return total, s.finish(ctx, entity, total, err)
		}
		if !more {
			return total, s.finish(ctx, entity, total, nil)
		}
	}
}

// finish records the outcome and returns the original error, so a bookkeeping
// failure cannot mask the crawl's own.
func (s *Syncer) finish(ctx context.Context, entity string, items int, cause error) error {
	if err := s.store.FinishSync(ctx, s.endpoint, entity, items, cause); err != nil {
		s.logger.Error("record sync state", "entity", entity, "error", err)
	}
	if cause != nil {
		return fmt.Errorf("sync %s: %w", entity, cause)
	}
	return nil
}
