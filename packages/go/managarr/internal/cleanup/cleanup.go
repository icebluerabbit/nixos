// Package cleanup removes torrents the tracker has unregistered.
//
// An unregistered torrent can never be announced again. Seeding it moves no
// bytes, earns no ratio, and holds a slot in the client forever — so it is
// dead weight the moment the tracker drops the record.
//
// Which messages mean "gone" is the qbittorrent client's to know rather than
// this package's: the vocabulary is a property of what trackers send, and it
// already lives there beside the status codes it accompanies. Matching on the
// message and not the status code alone matters — code 5 is a generic tracker
// error and covers things that are not deletions, including a peer-limit notice
// from a tracker that is working perfectly.
//
// This is the only thing in managarr that MUTATES anything. Everything else
// reads and records; the package comment on managarr says so, and this is the
// deliberate exception. It exists because managarr is the only component that
// can see a tracker's verdict and the torrent it refers to at the same time.
//
// Deletes the data along with the torrent. That is safe for anything else
// holding the same content: an *arr import and a cross-seed inject are
// hardlinks, so unlinking qBittorrent's copy leaves theirs in place and the
// bytes where they are. The data only actually goes when no other link exists,
// which is exactly the case where nothing else wanted it.
package cleanup

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"homelab/managarr/internal/model"
	"homelab/managarr/internal/query"

	"homelab/clients/qbittorrent"
)

// Candidate is one torrent a tracker has dropped.
type Candidate struct {
	Infohash string
	Tracker  string
	Message  string
	// ContentPath is what the torrent occupies on disk, and the only
	// human-readable identifier client_torrents carries — it holds no title.
	ContentPath string
	Category    string
	// SizeBytes is what the client downloaded, which is what the torrent
	// occupies in its own save path.
	SizeBytes int64
}

// Cleaner finds and removes unregistered torrents.
type Cleaner struct {
	q      *query.Query
	client *qbittorrent.Client
	logger *slog.Logger
	// clientName scopes the query, so a second torrent client's rows are not
	// deleted through the wrong API.
	clientName string
}

// New builds a cleaner.
func New(db *gorm.DB, c *qbittorrent.Client, logger *slog.Logger, clientName string) *Cleaner {
	return &Cleaner{q: query.Use(db), client: c, logger: logger, clientName: clientName}
}

// Stats reports what a run did.
type Stats struct {
	// Candidates is how many torrents matched.
	Candidates int
	// Deleted is how many were actually removed. Zero on a dry run.
	Deleted int
	// Unmatched is tracker errors this did not recognise as a deletion.
	// Worth surfacing: each one is either a phrasing missing from the list
	// above or a real error that is not a deletion at all.
	Unmatched int
	Bytes     int64
	DryRun    bool
	Took      time.Duration
}

// Run finds unregistered torrents and, unless dryRun, removes them with their
// data.
//
// Dry run is the default at every layer above this for a reason: the action is
// irreversible and the evidence for it is a string a tracker sent.
func (c *Cleaner) Run(ctx context.Context, dryRun bool) (Stats, error) {
	st := Stats{DryRun: dryRun}
	started := time.Now()

	candidates, unmatched, err := c.find(ctx)
	if err != nil {
		return st, err
	}
	st.Candidates = len(candidates)
	st.Unmatched = len(unmatched)

	for _, u := range unmatched {
		// Logged individually rather than counted, because the message IS the
		// actionable part: it either belongs in unregisteredPhrases or it is a
		// tracker problem worth seeing.
		c.logger.InfoContext(ctx, "unrecognised tracker error",
			"infohash", u.Infohash, "tracker", u.Tracker, "message", u.Message)
	}

	if len(candidates) == 0 {
		st.Took = time.Since(started)
		return st, nil
	}

	hashes := make([]string, 0, len(candidates))
	for _, cand := range candidates {
		st.Bytes += cand.SizeBytes
		hashes = append(hashes, cand.Infohash)
		c.logger.InfoContext(ctx, "unregistered torrent",
			"infohash", cand.Infohash, "path", cand.ContentPath,
			"tracker", cand.Tracker, "message", cand.Message,
			"category", cand.Category, "bytes", cand.SizeBytes,
			"action", map[bool]string{true: "would delete", false: "deleting"}[dryRun])
	}

	if dryRun {
		st.Took = time.Since(started)
		return st, nil
	}

	// Deleted with data. See the package comment: a hardlink held by an *arr
	// or a cross-seed survives this, so the bytes only go when nothing else
	// has a link to them.
	if err := c.client.Delete(ctx, hashes, true); err != nil {
		return st, err
	}
	st.Deleted = len(hashes)

	// Recorded AFTER the client confirmed, so nothing claims a deletion that
	// did not happen — and before returning, so a crash here leaves a torrent
	// deleted with no record rather than a record with no deletion. The first
	// is recoverable by observation; the second is a lie in the database.
	if err := c.record(ctx, candidates); err != nil {
		// The torrents are already gone. Failing the run now would report an
		// error for work that succeeded and invite a retry that finds nothing
		// to do — so this is logged loudly and the run still reports what it
		// deleted.
		c.logger.ErrorContext(ctx, "deleted torrents but could not record them",
			"count", len(candidates), "err", err)
	}

	// client_torrents keeps its rows otherwise. The next reconcile observes
	// the client without them and marks them removed, which keeps one writer
	// for that table rather than two disagreeing about what is present.
	st.Took = time.Since(started)
	return st, nil
}

// record writes down what was deleted and why.
//
// Two places, because they answer different questions. torrent_removals is the
// durable evidence — it outlives the torrent, the log and the client_torrents
// row. removal_reason on client_torrents is what a join reaches without a
// second lookup, which is what deciding whether a library file is orphaned
// needs.
//
// A reconcile pass cannot reconstruct either. It sees only that a torrent is
// absent, and that observation is identical whether the tracker unregistered
// it, a *arr cleaned it up, or somebody removed it by hand.
func (c *Cleaner) record(ctx context.Context, candidates []Candidate) error {
	now := time.Now().UTC()

	removals := make([]*model.TorrentRemoval, 0, len(candidates))
	hashes := make([]string, 0, len(candidates))
	for _, cand := range candidates {
		size := cand.SizeBytes
		removals = append(removals, &model.TorrentRemoval{
			Infohash:   cand.Infohash,
			ClientName: c.clientName,
			Reason:     ReasonUnregistered,
			Tracker:    cand.Tracker,
			Message:    cand.Message,
			Path:       strPtr(cand.ContentPath),
			SizeBytes:  &size,
			// Always, for this cleaner. See Run.
			DeletedData: true,
			RemovedAt:   now,
		})
		hashes = append(hashes, cand.Infohash)
	}

	if err := c.q.TorrentRemoval.WithContext(ctx).Create(removals...); err != nil {
		return fmt.Errorf("cleanup: record removals: %w", err)
	}

	// removed_at is left to the reconcile, which owns it. Only the reason is
	// written here, because only this code knows it.
	ct := c.q.ClientTorrent
	_, err := ct.WithContext(ctx).
		Where(ct.ClientName.Eq(c.clientName), ct.TorrentInfohash.In(hashes...)).
		Update(ct.RemovalReason, ReasonUnregistered)
	if err != nil {
		return fmt.Errorf("cleanup: mark removal reason: %w", err)
	}
	return nil
}

// ReasonUnregistered is managarr's own word for "the tracker dropped it".
//
// Its own vocabulary rather than the tracker's wording, which varies per
// tracker and is kept verbatim alongside it in torrent_removals.Message.
const ReasonUnregistered = "unregistered"

// strPtr returns nil for the empty string, matching the convention the rest of
// this model uses: absent is NULL, not "".
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// find returns the torrents to delete and the tracker errors it did not
// recognise.
//
// Reads the LAST OBSERVED state rather than asking qBittorrent again: the
// reconcile pass already stat'd every torrent's trackers, and repeating that
// here would both duplicate the work and open a window where the two disagree.
func (c *Cleaner) find(ctx context.Context) (candidates, unmatched []Candidate, err error) {
	tt := c.q.TorrentTracker
	ct := c.q.ClientTorrent

	// Scanned into pointers because three of these columns are nullable and a
	// NULL into a string is an error rather than an empty value. Normalized
	// below, which is the same job COALESCE used to do and is now visible.
	var rows []struct {
		Infohash    string
		Tracker     string
		Message     *string
		ContentPath *string
		Category    *string
		SizeBytes   *int64
	}
	err = tt.WithContext(ctx).
		Select(
			ct.TorrentInfohash.As("infohash"),
			tt.TrackerAnnounceHost.As("tracker"),
			tt.Message.As("message"),
			ct.ContentPath.As("content_path"),
			ct.Category.As("category"),
			ct.DownloadedBytes.As("size_bytes"),
		).
		Join(ct,
			ct.TorrentInfohash.EqCol(tt.TorrentInfohash),
			ct.RemovedAt.IsNull(),
			ct.ClientName.Eq(c.clientName),
		).
		Where(
			tt.StatusCode.Eq(qbittorrent.TrackerError),
			tt.Message.IsNotNull(),
		).
		Scan(&rows)
	if err != nil {
		return nil, nil, fmt.Errorf("cleanup: find candidates: %w", err)
	}

	all := make([]Candidate, 0, len(rows))
	for _, r := range rows {
		cand := Candidate{Infohash: r.Infohash, Tracker: r.Tracker}
		if r.Message != nil {
			cand.Message = *r.Message
		}
		if r.ContentPath != nil {
			cand.ContentPath = *r.ContentPath
		}
		if r.Category != nil {
			cand.Category = *r.Category
		}
		if r.SizeBytes != nil {
			cand.SizeBytes = *r.SizeBytes
		}
		all = append(all, cand)
	}

	// A torrent announcing to several trackers has a row per tracker, and one
	// tracker dropping it is enough — the others cannot resurrect a record
	// this tracker no longer has. Deduplicated so a torrent is deleted once.
	seen := make(map[string]struct{}, len(all))
	for _, r := range all {
		if !qbittorrent.IsUnregistered(r.Message) {
			unmatched = append(unmatched, r)
			continue
		}
		if _, dup := seen[r.Infohash]; dup {
			continue
		}
		seen[r.Infohash] = struct{}{}
		candidates = append(candidates, r)
	}
	return candidates, unmatched, nil
}
