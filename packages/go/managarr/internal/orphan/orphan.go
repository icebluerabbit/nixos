// Package orphan removes library entries nothing is seeding any more.
//
// The second half of what cleanup starts. Cleanup deletes a torrent its tracker
// unregistered, and the library file survives it — an import is a hardlink, so
// unlinking the client's copy leaves the *arr's. That is correct while any
// torrent still provides the content, and pointless once none does: the file is
// then something nothing announces, nothing seeds and nothing can replace,
// occupying space on the strength of a torrent that no longer exists.
//
// "Nothing provides it" is the whole difficulty, and the reason this waited for
// torrent_removals to exist. A library file names ONE torrent, the one it was
// imported from. Cross-seed then injects the same content from other trackers,
// and those are equally real providers — deleting the entry because its named
// torrent died would throw away a file three other trackers are still seeding.
//
// TWO INDEPENDENT TESTS, because one was not enough.
//
// The first is cross-seed's own record: it reports the torrent it searched FROM
// and the infohashes it found, and the ingest stores that pair on offers as
// SourceInfohash and Infohash. Following those edges in both directions gives
// the set of torrents carrying this content.
//
// That test is knowably incomplete. 70.6% of cross_seed_candidate offers carry
// no source_infohash — cross-seed's dataDir-mode searches have no searchee
// torrent to name, so the ingest records them edgeless deliberately — and
// thousands of the infohashes in those edgeless rows are live torrents the walk
// cannot reach. Relying on it alone proposed deleting five library files while
// twelve live torrents were seeding their exact bytes.
//
// The second test closes that: the reconcile stats every torrent file and every
// library file and records the inode it found, so asking whether a live
// torrent's file and this library file name the same (inode, size) answers
// "is anything still providing these bytes" directly. Recorded data, compared
// as data — it counts no links and asks the filesystem nothing.
//
// Still no name matching and no guessing from titles: every signal here is an
// identifier some other component wrote down.
//
// SONARR AND RADARR ONLY, by construction rather than configuration. Music
// arrives through Soulseek and beets as well as torrents, so a Lidarr album
// with no live torrent is an ordinary album and the reasoning here does not
// hold for it.
package orphan

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"homelab/clients/arr"
	"homelab/managarr/internal/model"
	"homelab/managarr/internal/query"
)

// services is which *arrs this acts on. See the package comment.
var services = []string{"sonarr", "radarr"}

// hops is how far the cross-seed neighbourhood is walked.
//
// One hop covers the ordinary shape — a torrent is searched from, candidates
// come back. The second covers a candidate later used as a searchee itself,
// which is how several cross-seeds of one content end up connected through a
// middle member rather than directly. Past two the edges stop meaning "the same
// content", so this bounds meaning rather than cost.
const hops = 2

// Candidate is one library file whose providers are all gone.
type Candidate struct {
	// Path is the file on disk, and the only part of this a human recognises.
	Path string
	// ServiceName is which *arr holds it, and so which client deletes it.
	ServiceName string
	// ArrFileID is the *arr's own id for the file. Without it there is nothing
	// to delete through: the API takes its own id and has no endpoint that
	// accepts a path.
	ArrFileID int64
	// Infohash is the torrent it was imported from, now removed.
	Infohash string
	// SizeBytes is what the file occupies.
	SizeBytes int64
	// Inode identifies the bytes themselves, paired with SizeBytes because
	// inodes are recycled after deletion — which is why file_data is keyed on
	// both. Zero when the stat pass has not reached this file.
	Inode int64
}

// Deleter is the piece of arr.Client this needs.
//
// An interface so the tests can refuse, count and misbehave without an *arr.
type Deleter interface {
	DeleteFile(ctx context.Context, fileID int64) error
}

// Remover finds and deletes orphaned library files.
type Remover struct {
	q      *query.Query
	logger *slog.Logger
	// clients is keyed by service name, which is what library_files carries.
	clients map[string]Deleter
}

// New builds a remover over the given clients, keyed by service name.
func New(db *gorm.DB, logger *slog.Logger, clients map[string]Deleter) *Remover {
	return &Remover{q: query.Use(db), logger: logger, clients: clients}
}

// NewFromArr is the production constructor, taking real clients.
func NewFromArr(db *gorm.DB, logger *slog.Logger, clients map[string]*arr.Client) *Remover {
	m := make(map[string]Deleter, len(clients))
	for name, c := range clients {
		m[name] = c
	}
	return New(db, logger, m)
}

// Stats reports what a run did.
type Stats struct {
	// Candidates is how many files had no provider left.
	Candidates int
	// Deleted is how many were removed. Zero on a dry run.
	Deleted int
	// Unroutable is candidates whose service has no configured client.
	// Surfaced rather than counted silently: each is a file this cannot act
	// on, and the reason is configuration, not data.
	Unroutable int
	// Failed is deletions the *arr refused.
	Failed int
	Bytes  int64
	DryRun bool
	Took   time.Duration
	// Files is what would go, or did. Carried in the result rather than left
	// to the log because this deletes media: a count is not enough to approve
	// on, and "which five" is the only question worth asking before applying.
	//
	// Capped at maxReported so one pathological run cannot answer with a
	// hundred megabytes of JSON; Candidates remains the true total.
	Files []Candidate `json:",omitempty"`
	// Truncated says Files is a prefix, so a short list is never mistaken for
	// the whole set.
	Truncated bool `json:",omitempty"`
}

// maxReported bounds the candidate list in a result.
//
// Generous on purpose: the expected size is single digits, and a run proposing
// more than this is itself the finding. Nobody should be scrolling past 500
// deletions to decide — they should be asking why there are 500.
const maxReported = 500

// Run finds orphaned library files and, unless dryRun, deletes them.
//
// Deleting removes the file from disk as well as from the *arr — see
// arr.DeleteFile. Dry run is the default above this for that reason.
//
// A failure on one file does not stop the others. Each delete is independent,
// and an *arr refusing one id says nothing about the next; aborting would make
// the run's effect depend on which candidate happened to sort first.
func (r *Remover) Run(ctx context.Context, dryRun bool) (Stats, error) {
	st := Stats{DryRun: dryRun}
	started := time.Now()

	candidates, err := r.find(ctx)
	if err != nil {
		return st, err
	}
	st.Candidates = len(candidates)
	st.Files = candidates
	if len(st.Files) > maxReported {
		st.Files = st.Files[:maxReported]
		st.Truncated = true
	}

	for _, c := range candidates {
		st.Bytes += c.SizeBytes
		client, ok := r.clients[c.ServiceName]
		if !ok {
			st.Unroutable++
			r.logger.WarnContext(ctx, "orphaned file with no client for its service",
				"path", c.Path, "service", c.ServiceName)
			continue
		}

		r.logger.InfoContext(ctx, "orphaned library file",
			"path", c.Path, "service", c.ServiceName, "file_id", c.ArrFileID,
			"infohash", c.Infohash, "bytes", c.SizeBytes,
			"action", map[bool]string{true: "would delete", false: "deleting"}[dryRun])

		if dryRun {
			continue
		}
		if err := client.DeleteFile(ctx, c.ArrFileID); err != nil {
			st.Failed++
			r.logger.ErrorContext(ctx, "delete failed",
				"path", c.Path, "service", c.ServiceName, "file_id", c.ArrFileID, "err", err)
			continue
		}
		st.Deleted++
	}

	// Nothing recorded here. Deleting through the *arr makes this an event in
	// the *arr's OWN history, and the history pass already turns that into
	// library_files.removed_at and removal_reason. A second record written from
	// this side would be a duplicate that disagrees the moment one is wrong.
	st.Took = time.Since(started)
	return st, nil
}

// find returns library files with no torrent left providing them.
func (r *Remover) find(ctx context.Context) ([]Candidate, error) {
	imports, err := r.importsFromRemovedTorrents(ctx)
	if err != nil {
		return nil, err
	}
	if len(imports) == 0 {
		return nil, nil
	}

	seeds := make([]string, 0, len(imports))
	for _, f := range imports {
		seeds = append(seeds, f.Infohash)
	}
	related, err := r.neighbourhood(ctx, seeds)
	if err != nil {
		return nil, err
	}

	live, err := r.liveInfohashes(ctx, related.all())
	if err != nil {
		return nil, err
	}

	seeded, err := r.inodesHeldByLiveTorrents(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]Candidate, 0, len(imports))
	for _, f := range imports {
		if related.anyLive(f.Infohash, live) {
			continue
		}
		// The second, independent test. See inodesHeldByLiveTorrents: the
		// cross-seed graph is knowably incomplete, and this catches what it
		// cannot see.
		if _, held := seeded[fileID{inode: f.Inode, size: f.SizeBytes}]; held && f.Inode != 0 {
			continue
		}
		out = append(out, f)
	}
	return out, nil
}

// fileID identifies bytes on disk.
//
// Inode AND size, because an inode is recycled once its last link goes. That
// is the same key file_data uses, for the same reason.
type fileID struct {
	inode int64
	size  int64
}

// inodesHeldByLiveTorrents is every file a live torrent currently holds,
// identified by the bytes rather than by a name or a hash.
//
// THE REASON THIS EXISTS. The cross-seed graph above is the primary test and it
// is knowably incomplete: 70.6% of cross_seed_candidate offers carry no
// source_infohash, because cross-seed's dataDir-mode searches have no searchee
// torrent to name and the ingest records them edgeless on purpose. Among those
// edgeless offers are thousands of infohashes that ARE live torrents, and the
// walk cannot reach any of them.
//
// That is not a depth problem — the five library files this was written for
// appear in zero non-self edges at any depth, so no number of hops would have
// saved them. Three of them had three separate live torrents seeding the very
// same inode as the library file while the walk concluded nothing provided it.
//
// This is recorded model data, not a filesystem question: the reconcile stats
// every torrent file and every library file and stores the inode it found. The
// join asks whether two rows name the same bytes. It does not count links and
// does not ask the filesystem anything.
func (r *Remover) inodesHeldByLiveTorrents(ctx context.Context) (map[fileID]struct{}, error) {
	tf := r.q.TorrentFile
	ct := r.q.ClientTorrent

	var rows []struct {
		FileDataInode     int64
		FileDataSizeBytes int64
	}
	err := tf.WithContext(ctx).
		Select(tf.FileDataInode, tf.FileDataSizeBytes).
		Join(ct, ct.TorrentInfohash.EqCol(tf.TorrentInfohash), ct.RemovedAt.IsNull()).
		Where(tf.FileDataInode.IsNotNull(), tf.FileDataSizeBytes.IsNotNull()).
		Scan(&rows)
	if err != nil {
		return nil, fmt.Errorf("orphan: inodes held by live torrents: %w", err)
	}

	out := make(map[fileID]struct{}, len(rows))
	for _, row := range rows {
		out[fileID{inode: row.FileDataInode, size: row.FileDataSizeBytes}] = struct{}{}
	}
	return out, nil
}

// importsFromRemovedTorrents lists live library files whose torrent managarr
// deleted.
//
// The starting set, and already narrow: everything after this only removes
// entries from it.
func (r *Remover) importsFromRemovedTorrents(ctx context.Context) ([]Candidate, error) {
	tr := r.q.TorrentRemoval
	var removed []string
	if err := tr.WithContext(ctx).Distinct(tr.Infohash).Pluck(tr.Infohash, &removed); err != nil {
		return nil, err
	}
	if len(removed) == 0 {
		return nil, nil
	}

	lf := r.q.LibraryFile
	files, err := lf.WithContext(ctx).
		Where(
			lf.RemovedAt.IsNull(),
			lf.ServiceName.In(services...),
			// Nothing to delete through without the *arr's own file id.
			lf.ArrFileID.IsNotNull(),
			lf.TorrentInfohash.In(removed...),
		).
		Order(lf.Path).
		Find()
	if err != nil {
		return nil, err
	}

	out := make([]Candidate, 0, len(files))
	for _, f := range files {
		// The Where above already excludes NULLs in all three, so these
		// dereferences are guarded by the query rather than by luck.
		c := Candidate{
			Path:      f.Path,
			ArrFileID: int64(*f.ArrFileID),
			Infohash:  *f.TorrentInfohash,
		}
		if f.ServiceName != nil {
			c.ServiceName = *f.ServiceName
		}
		// Size is genuinely optional: a file the stat pass has not reached yet
		// has no file_data row, and that is a missing measurement rather than
		// a reason to keep or drop it.
		if f.FileDataSizeBytes != nil {
			c.SizeBytes = *f.FileDataSizeBytes
		}
		if f.FileDataInode != nil {
			c.Inode = *f.FileDataInode
		}
		out = append(out, c)
	}
	return out, nil
}

// graph is the cross-seed neighbourhood of the seeds it was built from.
type graph struct {
	// reach maps a seed infohash to every infohash reachable from it, itself
	// included — the set of torrents that carry this content.
	reach map[string]map[string]struct{}
}

// all is every infohash mentioned anywhere in the graph.
func (g graph) all() []string {
	seen := make(map[string]struct{})
	for _, set := range g.reach {
		for h := range set {
			seen[h] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for h := range seen {
		out = append(out, h)
	}
	return out
}

// anyLive reports whether anything reachable from seed is still held.
func (g graph) anyLive(seed string, live map[string]struct{}) bool {
	for h := range g.reach[seed] {
		if _, ok := live[h]; ok {
			return true
		}
	}
	return false
}

// neighbourhood expands each seed across cross-seed's edges, hops deep.
//
// Expanded a level at a time rather than by loading the whole offers table:
// only infohashes reachable from a seed matter, and that is a few dozen rows
// against a table holding every release ever offered.
func (r *Remover) neighbourhood(ctx context.Context, seeds []string) (graph, error) {
	g := graph{reach: make(map[string]map[string]struct{}, len(seeds))}
	// A seed provides its own content, so it belongs to its own reachable set.
	// That is what makes a torrent with no cross-seeds at all fall out
	// correctly rather than as a special case.
	frontier := make(map[string]map[string]struct{}, len(seeds))
	for _, s := range seeds {
		g.reach[s] = map[string]struct{}{s: {}}
		frontier[s] = map[string]struct{}{s: {}}
	}

	for range hops {
		var lookup []string
		for _, set := range frontier {
			for h := range set {
				lookup = append(lookup, h)
			}
		}
		edges, err := r.edges(ctx, lookup)
		if err != nil {
			return graph{}, err
		}
		if len(edges) == 0 {
			break
		}

		next := make(map[string]map[string]struct{}, len(frontier))
		for seed, set := range frontier {
			for h := range set {
				for _, peer := range edges[h] {
					if _, known := g.reach[seed][peer]; known {
						continue
					}
					g.reach[seed][peer] = struct{}{}
					if next[seed] == nil {
						next[seed] = make(map[string]struct{})
					}
					next[seed][peer] = struct{}{}
				}
			}
		}
		if len(next) == 0 {
			break
		}
		frontier = next
	}
	return g, nil
}

// edges returns cross-seed's pairings for the given infohashes, undirected.
//
// Undirected because which torrent cross-seed searched FROM is an accident of
// which was grabbed first, and says nothing about which still exists.
func (r *Remover) edges(ctx context.Context, hashes []string) (map[string][]string, error) {
	if len(hashes) == 0 {
		return nil, nil
	}

	o := r.q.Offer
	offers, err := o.WithContext(ctx).
		Select(o.SourceInfohash, o.Infohash).
		Where(
			o.Kind.Eq(model.OfferCrossSeedCandidate),
			o.SourceInfohash.IsNotNull(),
			o.Infohash.IsNotNull(),
		).
		Where(o.WithContext(ctx).Or(o.SourceInfohash.In(hashes...)).Or(o.Infohash.In(hashes...))).
		Find()
	if err != nil {
		return nil, err
	}

	out := make(map[string][]string, len(offers)*2)
	for _, row := range offers {
		src, dst := *row.SourceInfohash, *row.Infohash
		out[src] = append(out[src], dst)
		out[dst] = append(out[dst], src)
	}
	return out, nil
}

// liveInfohashes is which of the given torrents a client still holds.
func (r *Remover) liveInfohashes(ctx context.Context, hashes []string) (map[string]struct{}, error) {
	if len(hashes) == 0 {
		return nil, nil
	}

	ct := r.q.ClientTorrent
	var held []string
	err := ct.WithContext(ctx).
		Distinct(ct.TorrentInfohash).
		Where(ct.RemovedAt.IsNull(), ct.TorrentInfohash.In(hashes...)).
		Pluck(ct.TorrentInfohash, &held)
	if err != nil {
		return nil, err
	}

	out := make(map[string]struct{}, len(held))
	for _, h := range held {
		out[h] = struct{}{}
	}
	return out, nil
}
