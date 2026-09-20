package reconcile

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"homelab/clients/qbittorrent"
	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// QBittorrent reconciles what the client holds.
//
// Writes two things per torrent: the torrent itself, which is true regardless
// of who holds it, and the client_torrent, which is this client's view of it —
// state, progress, ratio, paths. That split is why a torrent can be recorded
// from an announce or a cross-seed candidate without pretending some client has
// it.
type QBittorrent struct {
	client     *qbittorrent.Client
	dao        *dao.DAO
	lookups    *dao.Lookups
	logger     *slog.Logger
	clientName string
	now        func() time.Time
}

// NewQBittorrent builds the pass.
func NewQBittorrent(c *qbittorrent.Client, d *dao.DAO, l *dao.Lookups, logger *slog.Logger, clientName string) *QBittorrent {
	return &QBittorrent{
		client: c, dao: d, lookups: l, logger: logger,
		clientName: clientName, now: time.Now,
	}
}

// QBittorrentStats reports what a run did.
type QBittorrentStats struct {
	Torrents int
	Files    int
	Statted  int
	Trackers int
	Removed  int64
	Took     time.Duration
}

// Run performs one full pass.
func (r *QBittorrent) Run(ctx context.Context) (QBittorrentStats, error) {
	var st QBittorrentStats
	started := r.now()
	r.lookups.Reset()

	if err := dao.Upsert(ctx, r.dao, []model.Client{{
		Name:       r.clientName,
		Kind:       model.ClientQBittorrent,
		ObservedAt: timePtr(started),
	}}, []string{"name"}, []string{"kind", "observed_at"}); err != nil {
		return st, fmt.Errorf("qbittorrent: client: %w", err)
	}

	torrents, err := r.client.Torrents(ctx)
	if err != nil {
		return st, err
	}
	st.Torrents = len(torrents)

	seen := make([]string, 0, len(torrents))
	for _, t := range torrents {
		hash := strings.ToLower(t.Hash)
		seen = append(seen, hash)

		if err := r.writeTorrent(ctx, t, hash, started); err != nil {
			return st, fmt.Errorf("torrent %s: %w", hash, err)
		}

		n, statted, err := r.syncFiles(ctx, hash, t, started)
		if err != nil {
			return st, fmt.Errorf("files %s: %w", hash, err)
		}
		st.Files += n
		st.Statted += statted

		tn, err := r.syncTrackers(ctx, hash, t.Hash)
		if err != nil {
			return st, fmt.Errorf("trackers %s: %w", hash, err)
		}
		st.Trackers += tn
	}

	removed, err := r.markRemoved(ctx, seen, started)
	if err != nil {
		return st, err
	}
	st.Removed = removed

	st.Took = time.Since(started)
	return st, nil
}

// writeTorrent records the torrent and this client's view of it.
func (r *QBittorrent) writeTorrent(ctx context.Context, t qbittorrent.Torrent, hash string, now time.Time) error {
	releaseID, err := r.lookups.Release(ctx, t.Name)
	if err != nil {
		return err
	}
	// Ensures the tracker row exists; the torrent-to-tracker links are written
	// per-tracker by syncTrackers.
	if _, err := r.lookups.Tracker(ctx, qbittorrent.AnnounceHost(t.Tracker)); err != nil {
		return err
	}

	// The torrent itself: facts that hold no matter who has it.
	if err := dao.Upsert(ctx, r.dao, []model.Torrent{{
		Infohash:       hash,
		ReleaseID:      releaseID,
		InfohashV1:     lowerPtr(t.InfoHashV1),
		InfohashV2:     lowerPtr(t.InfoHashV2),
		Private:        t.Private,
		TotalSizeBytes: int64Ptr(t.TotalSize),
		Comment:        strPtr(t.Comment),
		CreatedBy:      strPtr(t.CreatedBy),
		CreationDate:   timePtr(unixTime(t.CreationDate)),
		// Carries tracker keys in its tr= parameters; the model's BeforeSave
		// hook strips them while preserving the info hash.
		MagnetURI: strPtr(t.MagnetURI),
	}}, []string{"infohash"},
		[]string{"release_id", "infohash_v1", "infohash_v2", "private",
			"total_size_bytes", "comment", "created_by", "creation_date",
			"magnet_uri"},
	); err != nil {
		return err
	}

	// This client's view. removed_at is cleared: seeing the torrent now means
	// the client has it, whatever an earlier sweep concluded.
	return dao.Upsert(ctx, r.dao, []model.ClientTorrent{{
		ClientName:      r.clientName,
		TorrentInfohash: hash,
		State:           strPtr(t.State),
		// floatPtr treats zero as absent, which is right here: a torrent at 0%
		// has nothing of itself, and that is what NULL means for a value the
		// client only reports once metadata has arrived.
		Progress:           floatPtr(t.Progress),
		Ratio:              floatPtr(t.Ratio),
		UploadedBytes:      int64Ptr(t.Uploaded),
		DownloadedBytes:    int64Ptr(t.Downloaded),
		SeedingTimeSeconds: int64Ptr(t.SeedingTime),
		TimeActiveSeconds:  int64Ptr(t.TimeActive),
		Name:               strPtr(t.Name),
		SavePath:           strPtr(t.SavePath),
		ContentPath:        strPtr(t.ContentPath),
		Category:           strPtr(t.Category),
		Tags:               strPtr(t.Tags),
		AddedAt:            timePtr(unixTime(t.AddedOn)),
		CompletedAt:        timePtr(unixTime(t.CompletionOn)),
		LastActivityAt:     timePtr(unixTime(t.LastActivity)),
		RemovedAt:          nil,
	}}, []string{"client_name", "torrent_infohash"},
		[]string{"state", "progress", "ratio", "uploaded_bytes",
			"downloaded_bytes", "seeding_time_seconds", "time_active_seconds",
			// name is updated, not just inserted: a torrent renamed in the
			// client would otherwise keep the name it was first seen under,
			// and this column exists to be joined against what other tools
			// report — a stale one resolves to nothing.
			"name",
			"save_path", "content_path", "category", "tags", "added_at",
			"completed_at", "last_activity_at", "removed_at"},
	)
}

// syncFiles records the torrent's content and, where reachable, the inode
// behind each file.
//
// Files at PrioritySkip are listed by the API but never written to disk, so
// they are recorded without inode identity rather than stat'd and logged as
// missing.
func (r *QBittorrent) syncFiles(ctx context.Context, hash string, t qbittorrent.Torrent, now time.Time) (fileCount, linked int, err error) {
	files, err := r.client.Files(ctx, t.Hash)
	if err != nil {
		return 0, 0, err
	}

	statted := 0
	rows := make([]model.TorrentFile, 0, len(files))
	for _, f := range files {
		abs := filepath.Join(t.SavePath, f.Name)

		var inode, size *int64
		if f.Priority != qbittorrent.PrioritySkip {
			in, sz, err := statInto(ctx, r.dao, abs, now)
			if err != nil {
				return 0, statted, err
			}
			if in != nil {
				inode, size = in, sz
				statted++
			}
		}

		rows = append(rows, model.TorrentFile{
			TorrentInfohash:   hash,
			Path:              abs,
			FileDataInode:     inode,
			FileDataSizeBytes: size,
			IsArchive:         boolPtr(isArchive(f.Name)),
		})
	}

	// Updated rather than ignored: the inode is resolved by a stat that can
	// fail on one pass and succeed on the next, so a row first written without
	// one has to be able to gain it.
	if err := dao.Upsert(ctx, r.dao, rows,
		[]string{"torrent_infohash", "path"},
		[]string{"file_data_inode", "file_data_size_bytes", "is_archive"},
	); err != nil {
		return len(files), statted, fmt.Errorf("torrent files: %w", err)
	}
	return len(files), statted, nil
}

// syncTrackers replaces the torrent's tracker rows.
//
// Replace rather than upsert: a tracker dropped from a torrent should
// disappear, and the set is small. Both halves run in one transaction so the
// torrent is never briefly seen with no trackers.
func (r *QBittorrent) syncTrackers(ctx context.Context, hash, rawHash string) (int, error) {
	trackers, err := r.client.Trackers(ctx, rawHash)
	if err != nil {
		return 0, err
	}

	rows := make([]model.TorrentTracker, 0, len(trackers))
	for _, t := range trackers {
		host := qbittorrent.AnnounceHost(t.URL)
		if host == "" {
			// DHT/PeX/LSD pseudo-entries: not trackers.
			continue
		}
		if _, err := r.lookups.Tracker(ctx, host); err != nil {
			return 0, err
		}
		rows = append(rows, model.TorrentTracker{
			TorrentInfohash:     hash,
			TrackerAnnounceHost: host,
			// For a private tracker the announce URL IS the passkey; the
			// model's BeforeSave hook strips it.
			URL:        strPtr(t.URL),
			Status:     strPtr(statusName(t.Status)),
			StatusCode: intPtr(t.Status),
			Message:    strPtr(t.Msg),
		})
	}

	err = r.dao.Tx(ctx, func(tx *dao.DAO) error {
		tt := tx.Q().TorrentTracker
		if _, err := tt.WithContext(ctx).
			Where(tt.TorrentInfohash.Eq(hash)).
			Delete(); err != nil {
			return err
		}
		return dao.Create(ctx, tx, rows)
	})
	if err != nil {
		return 0, fmt.Errorf("trackers for %s: %w", hash, err)
	}
	return len(rows), nil
}

// minSeenFraction is the share of live rows a pass must observe before it is
// allowed to mark anything removed.
//
// A torrent genuinely disappearing is a trickle. A wholesale drop means the
// client answered wrongly, not that the library was emptied.
const minSeenFraction = 0.5

// markRemoved stamps removed_at on rows this client no longer reports.
//
// Refuses to act on a suspiciously small observation. qBittorrent answers 200
// with an empty array while it is still loading its session after a restart,
// and one such pass would otherwise mark every torrent removed.
func (r *QBittorrent) markRemoved(ctx context.Context, seen []string, now time.Time) (int64, error) {
	if len(seen) == 0 {
		r.logger.Warn("qbittorrent: client reported no torrents; skipping removal sweep")
		return 0, nil
	}

	ct := r.dao.Q().ClientTorrent
	live, err := ct.WithContext(ctx).
		Where(ct.ClientName.Eq(r.clientName), ct.RemovedAt.IsNull()).
		Count()
	if err != nil {
		return 0, fmt.Errorf("qbittorrent: count live: %w", err)
	}
	if live > 0 && float64(len(seen)) < float64(live)*minSeenFraction {
		r.logger.Error("qbittorrent: refusing removal sweep, far fewer torrents than are live",
			"seen", len(seen), "live", live)
		return 0, nil
	}

	res, err := ct.WithContext(ctx).
		Where(
			ct.ClientName.Eq(r.clientName),
			ct.RemovedAt.IsNull(),
			ct.TorrentInfohash.NotIn(seen...),
		).
		Update(ct.RemovedAt, now.UTC())
	if err != nil {
		return 0, fmt.Errorf("qbittorrent: mark removed: %w", err)
	}
	return res.RowsAffected, nil
}

// lowerPtr lowercases a hash, or nil when empty.
func lowerPtr(s string) *string {
	if s == "" {
		return nil
	}
	l := strings.ToLower(s)
	return &l
}

// isArchive reports whether a filename looks like an archive unpackerr would
// extract.
func isArchive(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".rar", ".zip", ".7z", ".gz", ".bz2", ".tar":
		return true
	}
	return false
}

// statusName decodes qBittorrent's tracker status enum.
func statusName(code int) string {
	switch code {
	case qbittorrent.TrackerDisabled:
		return "disabled"
	case qbittorrent.TrackerNotContacted:
		return "not_contacted"
	case qbittorrent.TrackerWorking:
		return "working"
	case qbittorrent.TrackerUpdating:
		return "updating"
	case qbittorrent.TrackerNotWorking:
		return "not_working"
	case qbittorrent.TrackerError:
		return "error"
	default:
		return "unknown"
	}
}
