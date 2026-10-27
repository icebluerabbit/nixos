// Package prune deletes slskd searches older than a retention window.
//
// slskd exposes retention settings for transfers, files, logs and the share
// cache, but none for searches: every search it has ever run stays in
// search.db forever. With a soularr-style consumer issuing a couple of hundred
// searches a day that database reaches gigabytes, and because the web UI's
// search view loads the entire /api/v0/searches listing in one response, the
// page stalls long before the API does. This is the missing retention pass,
// run from outside on a timer.
package prune

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"homelab/app"
	"homelab/clients/slskd"
	"homelab/httpx"
)

// Defaults for the command line.
const (
	// DefaultBase reaches slskd on loopback from inside its own container,
	// where its API needs no token.
	DefaultBase = "http://127.0.0.1:5030"
	// DefaultRetentionDays is deliberately short. Retention here bounds a
	// response the browser has to render, not an archive anybody reads:
	// searches older than a week have already been acted on or abandoned.
	DefaultRetentionDays = 7
	// DefaultWorkers bounds the deletes in flight. There is no bulk delete
	// route, so a first run against an unpruned instance issues tens of
	// thousands of requests and doing them one at a time takes hours.
	DefaultWorkers = 8
)

// maxReportedFailures caps how many individual errors a failed run repeats. A
// systemic failure produces one per stale search, and joining ten thousand of
// them would turn one bad run into an unreadable journal entry.
const maxReportedFailures = 5

// hoursPerDay expresses the retention window, which is given in days because
// that is the unit the operator thinks in.
const hoursPerDay = 24 * time.Hour

// Config is what the tool takes on the command line.
type Config struct {
	// Base is the slskd root URL.
	Base string `json:"base"`
	// RetentionDays is how old a search must be, by its start time, before
	// it is deleted.
	RetentionDays int `json:"retention_days"`
	// Workers is how many deletes may be in flight at once.
	Workers int `json:"workers"`
}

// ErrRetention reports a retention window that would delete everything.
var ErrRetention = errors.New("slskd-prune: retention-days must be at least 1")

// ErrWorkers reports a worker count that would do no work.
var ErrWorkers = errors.New("slskd-prune: workers must be at least 1")

// Defaults returns the configuration a unit that passes no flags gets.
func Defaults() Config {
	return Config{
		Base:          DefaultBase,
		RetentionDays: DefaultRetentionDays,
		Workers:       DefaultWorkers,
	}
}

// Validate rejects a configuration that would misbehave rather than fail.
//
// A zero or negative retention window is the dangerous one: it makes the cutoff
// now or later, so every search matches and the whole history goes. Refusing is
// better than deleting, since nothing here can put it back.
func (c Config) Validate() error {
	if c.RetentionDays < 1 {
		return ErrRetention
	}
	if c.Workers < 1 {
		return ErrWorkers
	}
	return nil
}

// Cutoff reports the instant before which a search is stale.
func (c Config) Cutoff(now time.Time) time.Time {
	return now.Add(-time.Duration(c.RetentionDays) * hoursPerDay)
}

// Stale selects the searches that started before cutoff.
//
// A search that has not finished is kept whatever its age. slskd reports a
// search as incomplete while it is still collecting responses, and deleting one
// mid-flight would drop results a caller is currently waiting on — an old start
// time is far more likely to mean a search that is wedged than one worth
// racing.
func Stale(searches []slskd.Search, cutoff time.Time) []slskd.Search {
	stale := make([]slskd.Search, 0, len(searches))
	for _, s := range searches {
		if s.IsComplete && s.StartedAt.Before(cutoff) {
			stale = append(stale, s)
		}
	}
	return stale
}

// Run fetches the search history and deletes everything past the window.
func Run(ctx context.Context, env *app.Env, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	client, err := slskd.New(httpx.Config{
		BaseURL:   cfg.Base,
		UserAgent: "slskd-prune",
		Logger:    env.Logger,
	})
	if err != nil {
		return err
	}

	searches, err := client.Searches(ctx)
	if err != nil {
		return err
	}

	cutoff := cfg.Cutoff(time.Now())
	stale := Stale(searches, cutoff)

	env.Logger.Info("search history",
		slog.Int("total", len(searches)),
		slog.Int("stale", len(stale)),
		slog.Time("cutoff", cutoff))

	if len(stale) == 0 {
		return nil
	}
	if env.DryRun {
		env.Logger.Info("dry run: not deleting", slog.Int("count", len(stale)))
		return nil
	}

	deleted, failures := deleteAll(ctx, client, stale, cfg.Workers)
	env.Logger.Info("pruned searches",
		slog.Int("deleted", deleted),
		slog.Int("failed", len(failures)))

	if err := ctx.Err(); err != nil {
		return err
	}
	if len(failures) > 0 {
		return fmt.Errorf("slskd-prune: %d of %d deletions failed: %w",
			len(failures), len(stale),
			errors.Join(failures[:min(len(failures), maxReportedFailures)]...))
	}
	return nil
}

// deleteAll removes every stale search, at most workers at a time, and reports
// how many went and what failed.
//
// A single failure does not stop the run. The common one is a 404 from a search
// deleted between the listing and the delete, and abandoning several thousand
// good deletions over one of those would mean the backlog never drains.
func deleteAll(
	ctx context.Context,
	client *slskd.Client,
	stale []slskd.Search,
	workers int,
) (int, []error) {
	var (
		mu       sync.Mutex
		failures []error
		deleted  atomic.Int64
	)

	group := new(errgroup.Group)
	group.SetLimit(workers)

	for _, s := range stale {
		group.Go(func() error {
			// Cancellation is reported by the caller from ctx, so a
			// stopped run neither records failures nor returns early
			// out of the group.
			if ctx.Err() != nil {
				return nil
			}
			if err := client.DeleteSearch(ctx, s.ID); err != nil {
				mu.Lock()
				failures = append(failures, err)
				mu.Unlock()
				return nil
			}
			deleted.Add(1)
			return nil
		})
	}
	// Every goroutine returns nil; the errors that matter are in failures.
	_ = group.Wait()

	return int(deleted.Load()), failures
}
