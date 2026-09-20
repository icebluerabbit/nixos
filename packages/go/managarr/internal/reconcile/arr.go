package reconcile

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"homelab/clients/arr"
	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// Arr reconciles one Servarr instance: Sonarr, Radarr or Lidarr.
//
// These three are the authority on what the library contains and on what it is
// meant to contain. Every other service here has an opinion about a file; the
// *arr decided the file should exist, chose the release that produced it, and
// is the only thing that knows which piece of content it belongs to.
//
// The pass runs in dependency order and that order is not incidental:
//
//	catalog  content and its external ids   — everything else references content
//	files    what is on disk right now      — references content
//	history  how each of those arrived      — references content and files
//	queue    what is in flight              — references content
//
// History is walked last and oldest-first. It is retrospective, so one walk
// recovers everything that happened before this service existed, which is the
// entire reason a backfill is possible at all.
type Arr struct {
	client *arr.Client
	// walk reads history through this instance's sidecar with a stored
	// cursor. Nil means no sidecar is configured and history comes from the
	// *arr directly — which is correct but re-reads everything every pass.
	walk        *SidecarWalk
	dao         *dao.DAO
	lookups     *dao.Lookups
	logger      *slog.Logger
	serviceName string
	pageSize    int
}

// NewArr builds the pass. A nil walk reads history from the *arr directly.
func NewArr(c *arr.Client, walk *SidecarWalk, d *dao.DAO, l *dao.Lookups, logger *slog.Logger, serviceName string, pageSize int) *Arr {
	if pageSize <= 0 {
		pageSize = 500
	}
	return &Arr{
		client: c, walk: walk, dao: d, lookups: l, logger: logger,
		serviceName: serviceName, pageSize: pageSize,
	}
}

// ArrStats reports what a run did.
type ArrStats struct {
	Contents    int
	Files       int
	Renamed     int
	Grabs       int
	Imports     int
	Failures    int
	Removed     int
	QueueItems  int
	Blocklisted int
	RootFolders int
	Health      int
	// Resumed is the history cursor this pass started from, empty on a first
	// run or when no sidecar is configured. Reported because "0 grabs" means
	// the opposite thing depending on it: caught up, or never started.
	Resumed string
	Took    time.Duration
}

// Run executes one full pass.
func (a *Arr) Run(ctx context.Context) (ArrStats, error) {
	var st ArrStats
	started := time.Now()
	a.lookups.Reset()

	if err := a.syncService(ctx); err != nil {
		return st, err
	}

	n, err := a.syncRootFolders(ctx)
	if err != nil {
		return st, err
	}
	st.RootFolders = n

	if st.Health, err = a.syncHealth(ctx); err != nil {
		return st, err
	}

	index, err := a.syncCatalog(ctx)
	if err != nil {
		return st, err
	}
	st.Contents = index.count()

	// History is read once and used twice, because renames have to be applied
	// BEFORE the file snapshot and everything else in history has to come
	// after it.
	//
	// The ordering is not cosmetic. library_files.path is the unique key and
	// the join target for import provenance and for Bazarr's subtitles. If a
	// rename is applied after the snapshot, the snapshot has already inserted a
	// second row at the new path — the old row keeps the infohash, the new one
	// has none, and the provenance is silently detached with both rows looking
	// perfectly valid.
	//
	// It comes through the sidecar when one is configured, so a steady-state
	// pass sees the handful of records from the last hour rather than replaying
	// all 28,000; the *arr's own /history takes a page and a sort and offers no
	// filter to do that with. The read is atomic for exactly the ordering above:
	// the two phases straddle the file snapshot, and a cursor advancing between
	// them would leave records nothing ever wrote behind a cursor claiming they
	// were read.
	read, err := a.history(ctx)
	if err != nil {
		return st, err
	}

	st.Resumed, err = read.commit(ctx, func(ctx context.Context, records []arr.HistoryRecord) error {
		if st.Renamed, err = a.applyRenames(ctx, records); err != nil {
			return err
		}

		if st.Files, err = a.syncFiles(ctx, index); err != nil {
			return err
		}

		grabs, imports, failures, removed, err := a.syncHistory(ctx, index, records)
		if err != nil {
			return err
		}
		st.Grabs, st.Imports, st.Failures, st.Removed = grabs, imports, failures, removed
		return nil
	})
	if err != nil {
		return st, err
	}

	if st.QueueItems, err = a.syncQueue(ctx, index); err != nil {
		return st, err
	}
	if st.Blocklisted, err = a.syncBlocklist(ctx, index); err != nil {
		return st, err
	}

	st.Took = time.Since(started)
	return st, nil
}

// syncService records the instance and its version.
//
// Run first, and not only for the version: every table this pass writes has a
// foreign key to services, so the row has to exist before anything else does.
func (a *Arr) syncService(ctx context.Context) error {
	version, err := a.client.Version(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return dao.Upsert(ctx, a.dao, []model.Service{{
		Name:       a.serviceName,
		Kind:       string(a.client.Kind()),
		Version:    strPtr(version),
		LastSeenAt: &now,
	}}, []string{"name"}, []string{"kind", "version", "last_seen_at"})
}

// syncRootFolders records the library roots and their free space.
func (a *Arr) syncRootFolders(ctx context.Context) (int, error) {
	list, err := a.client.RootFolders(ctx)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	rows := make([]model.RootFolder, 0, len(list))
	for _, rf := range list {
		rows = append(rows, model.RootFolder{
			ServiceName: a.serviceName,
			Path:        rf.Path,
			FreeBytes:   int64Ptr(rf.FreeSpace),
			TotalBytes:  int64Ptr(rf.TotalSpace),
			Accessible:  boolPtr(rf.Accessible),
			ObservedAt:  now,
		})
	}
	if err := dao.Upsert(ctx, a.dao, rows, []string{"service_name", "path"},
		[]string{"free_bytes", "total_bytes", "accessible", "observed_at"}); err != nil {
		return 0, fmt.Errorf("%s: root folders: %w", a.serviceName, err)
	}
	return len(rows), nil
}

// syncHealth records the instance's active complaints.
//
// first_seen_at is deliberately not in the update list. A check that has been
// failing for a week should still say so on the hundredth pass; overwriting it
// every time would make every problem look brand new.
func (a *Arr) syncHealth(ctx context.Context) (int, error) {
	list, err := a.client.Health(ctx)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	rows := make([]model.HealthEvent, 0, len(list))
	for _, h := range list {
		rows = append(rows, model.HealthEvent{
			ServiceName: a.serviceName,
			Type:        h.Type,
			Source:      h.Source,
			Message:     strPtr(h.Message),
			FirstSeenAt: now,
			LastSeenAt:  now,
		})
	}
	if err := dao.Upsert(ctx, a.dao, rows,
		[]string{"service_name", "type", "source"},
		[]string{"message", "last_seen_at"}); err != nil {
		return 0, fmt.Errorf("%s: health: %w", a.serviceName, err)
	}
	return len(rows), nil
}
