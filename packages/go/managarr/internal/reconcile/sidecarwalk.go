package reconcile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"homelab/managarr/internal/dao"
	"homelab/sidecar"
)

// SidecarWalk reads one sidecar's /changes from where the last pass stopped.
//
// The cursor lifecycle lives here rather than in each pass because getting it
// wrong is silent in both directions: a cursor saved too early loses everything
// the failing page held, and one never saved turns every pass back into a full
// re-walk while still reporting success.
//
// The rule it enforces is that the cursor only advances past a page the handler
// accepted. A handler that returns an error stops the walk with the cursor
// still pointing at the start of the page that failed, so the next pass re-reads
// it. Re-reading is safe — every write downstream is an idempotent upsert — and
// skipping is not.
type SidecarWalk struct {
	client   *sidecar.Client
	dao      *dao.DAO
	logger   *slog.Logger
	source   string
	pageSize int
}

// NewSidecarWalk builds a walker over one sidecar.
func NewSidecarWalk(c *sidecar.Client, d *dao.DAO, logger *slog.Logger, source string, pageSize int) *SidecarWalk {
	if pageSize <= 0 {
		pageSize = 1000
	}
	return &SidecarWalk{
		client: c, dao: d, logger: logger,
		source: source, pageSize: pageSize,
	}
}

// WalkStats reports what one walk did.
type WalkStats struct {
	// Events is how many were read and handled this pass. Zero is the normal
	// steady state: it means nothing new happened upstream.
	Events int
	// Pages is how many requests the sidecar answered.
	Pages int
	// Resumed is where the walk started, and empty on a first run or after a
	// reset. Logged because "this pass read 28,000 events" means something
	// very different depending on it.
	Resumed string
	Took    time.Duration
}

// Run walks from the stored cursor, calling fn per page.
//
// fn receives events in the order the sidecar returned them, which is the
// sidecar's own and not necessarily chronological — see sidecar.Service.Changes.
func (w *SidecarWalk) Run(ctx context.Context, fn func(context.Context, []sidecar.Event) error) (WalkStats, error) {
	var st WalkStats
	started := time.Now()

	cursor, err := w.load(ctx)
	if err != nil {
		return st, err
	}
	st.Resumed = cursor

	var (
		handlerErr error
		newest     *time.Time
	)

	// The cursor is saved per page rather than once at the end, and the one
	// Changes returns is discarded for that reason. A pass over a backlog
	// takes minutes and can be killed by a deploy partway through; saving only
	// at the end would throw away every page it managed.
	_, err = w.client.Changes(ctx, cursor, w.pageSize, func(page []sidecar.Event) bool {
		if err := fn(ctx, page); err != nil {
			handlerErr = err
			return false
		}
		st.Events += len(page)
		st.Pages++

		for _, e := range page {
			if !e.OccurredAt.IsZero() && (newest == nil || e.OccurredAt.After(*newest)) {
				at := e.OccurredAt
				newest = &at
			}
		}

		next := page[len(page)-1].Cursor
		if err := w.save(ctx, next, newest, len(page)); err != nil {
			handlerErr = err
			return false
		}

		w.logger.InfoContext(ctx, "sidecar progress",
			"source", w.source, "events", st.Events, "pages", st.Pages,
			"cursor", next)
		return true
	})

	// handlerErr first: Changes reports the walk stopped cleanly when fn
	// returns false, so the real reason would otherwise be swallowed.
	if handlerErr != nil {
		return st, handlerErr
	}
	if err != nil {
		return st, err
	}

	st.Took = time.Since(started)
	return st, nil
}

// RunAtomic reads everything after the cursor, hands it over once, and only
// then advances.
//
// For passes that cannot do their work page by page. The *arr pass is the
// reason: it consumes one history walk TWICE, applying renames before it
// snapshots files and recording the history after, because a rename applied
// after the snapshot leaves two library_file rows — the old one holding the
// infohash, the new one holding none — and detaches the provenance with both
// rows looking valid.
//
// Run cannot express that. Its cursor advances per page, so a failure in the
// second phase would leave the cursor past records nothing ever wrote, which is
// the silent skip this whole type exists to prevent.
//
// The cost is real and is the reason this is not the default: a pass killed
// partway through keeps nothing, so a first backfill that dies at record 27,000
// starts again at zero. Correctness over progress — a repeated read is free,
// and a skipped one is permanent.
func (w *SidecarWalk) RunAtomic(ctx context.Context, fn func(context.Context, []sidecar.Event) error) (WalkStats, error) {
	var st WalkStats
	started := time.Now()

	cursor, err := w.load(ctx)
	if err != nil {
		return st, err
	}
	st.Resumed = cursor

	var all []sidecar.Event
	final, err := w.client.Changes(ctx, cursor, w.pageSize, func(page []sidecar.Event) bool {
		all = append(all, page...)
		st.Pages++
		w.logger.InfoContext(ctx, "sidecar collecting",
			"source", w.source, "events", len(all), "pages", st.Pages)
		return true
	})
	if err != nil {
		return st, err
	}

	// Nothing new. Not an error and not worth a write: saving an unchanged
	// cursor every hour would make updated_at useless as a signal.
	if len(all) == 0 {
		st.Took = time.Since(started)
		return st, nil
	}

	if err := fn(ctx, all); err != nil {
		return st, err
	}
	st.Events = len(all)

	var newest *time.Time
	for _, e := range all {
		if !e.OccurredAt.IsZero() && (newest == nil || e.OccurredAt.After(*newest)) {
			at := e.OccurredAt
			newest = &at
		}
	}
	if err := w.save(ctx, final, newest, len(all)); err != nil {
		return st, err
	}

	st.Took = time.Since(started)
	return st, nil
}

// load reads the stored cursor. A source never walked before starts empty,
// which is not an error — it is every source's first pass.
func (w *SidecarWalk) load(ctx context.Context) (string, error) {
	sc := w.dao.Q().SidecarCursor
	row, err := sc.WithContext(ctx).Where(sc.Source.Eq(w.source)).Take()
	switch {
	case err == nil:
		return row.Cursor, nil
	case errorIsNotFound(err):
		return "", nil
	default:
		return "", fmt.Errorf("sidecar cursor %s: load: %w", w.source, err)
	}
}

// save advances the cursor and its diagnostics in one statement.
//
// events_read accumulates rather than being overwritten, so a pass that reads
// nothing leaves the total intact — which is what makes "has this source ever
// produced anything" answerable.
func (w *SidecarWalk) save(ctx context.Context, cursor string, newest *time.Time, read int) error {
	// Generated from an annotated statement — see _gen/querier.go. Both the
	// accumulating counter and the coalesced timestamp are functions of what is
	// already stored, so they are evaluated inside the statement rather than
	// read, adjusted and written back.
	err := w.dao.Q().SidecarCursor.WithContext(ctx).
		Advance(w.source, cursor, newest, int64(read), time.Now().UTC())
	if err != nil {
		return fmt.Errorf("sidecar cursor %s: save: %w", w.source, err)
	}
	return nil
}

// Reset clears a source's cursor so the next pass re-walks from the beginning.
//
// Exists because the alternative — deleting a row by hand in psql — is what
// someone does at 2am when a schema change means the whole history has to be
// re-read, and doing it through the model keeps the counters honest.
func (w *SidecarWalk) Reset(ctx context.Context) error {
	sc := w.dao.Q().SidecarCursor
	if _, err := sc.WithContext(ctx).Where(sc.Source.Eq(w.source)).Delete(); err != nil {
		return fmt.Errorf("sidecar cursor %s: reset: %w", w.source, err)
	}
	return nil
}

// errorIsNotFound reports GORM's empty-result sentinel.
func errorIsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
