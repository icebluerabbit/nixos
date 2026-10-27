// Package reap finds everything the stack has left behind.
//
// The two existing sweeps each answer one question. cleanup removes torrents a
// tracker has unregistered; orphan removes library entries no torrent provides
// any more. Between and around them sit several other ways for something to be
// stranded, and nothing was looking for any of them:
//
//	download data    a torrent removed WITHOUT its data — the common result of
//	                 deleting one by hand, and invisible to managarr because
//	                 the torrent is simply gone from the client
//	cross-seed links a link tree cross-seed built for a torrent that no longer
//	                 exists; hardlinks, so dead weight rather than lost space
//	incomplete       a partial download for a torrent that is no longer there
//	torrent data     a torrent the client still holds whose files are gone from
//	                 disk, which seeds nothing and never will again
//	library file     an *arr row pointing at a file that is not there
//
// The first three are disk-side: something exists that nothing claims, and
// those are the ones this DELETES when applying. The last two are the inverse —
// something is claimed that does not exist — so there is nothing on disk to
// remove and they stay diagnostics. Both directions are worth knowing and
// neither is visible from one side alone.
//
// Dry run by default, like the sweeps either side of it. The evidence for
// removing an entry is the ABSENCE of a claim, and absence is what a stale or
// partial read produces — so the claim set is read from the client rather than
// the database, and read a second time immediately before anything is unlinked.
//
// EVERYTHING HERE IS FOUND BY EXACT PATH. A reported path is folded up to the
// entry containing it and compared with what is on disk; see paths.go, which
// carries the argument for why that fold is not optional. No name matching, no
// inode comparison, no link counting — those all answer "does this resemble
// that", and the question here is "is this exact thing claimed".
package reap

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"slices"
	"time"

	"gorm.io/gorm"

	"homelab/clients/qbittorrent"
	"homelab/managarr/internal/query"
)

// Class is one kind of leftover.
type Class string

const (
	// ClassDownloadData is data in the download tree that no torrent claims.
	ClassDownloadData Class = "download_data"
	// ClassCrossSeedLinks is a cross-seed link tree with no torrent behind it.
	ClassCrossSeedLinks Class = "cross_seed_links"
	// ClassIncomplete is a partial download with no torrent behind it.
	ClassIncomplete Class = "incomplete"
	// ClassTorrentDataMissing is a torrent whose data is gone from disk.
	ClassTorrentDataMissing Class = "torrent_data_missing"
	// ClassLibraryFileMissing is an *arr row whose file is gone from disk.
	ClassLibraryFileMissing Class = "library_file_missing"
)

// Finding is one stranded thing.
type Finding struct {
	Class Class
	Path  string
	// Bytes is what it occupies, where that is known and meaningful. Zero for
	// the two inverse classes: there is nothing on disk to measure.
	Bytes int64
	// Category is the download category or cross-seed tracker it sits under,
	// which is what decides whether anything else could be holding the bytes.
	Category string `json:",omitempty"`
	// Detail carries whatever else identifies it — an infohash, a service.
	Detail string `json:",omitempty"`
}

// Config says where to look.
type Config struct {
	// CompleteRoot is the download tree, laid out as <category>/<entry>.
	CompleteRoot string
	// LinksRoot is cross-seed's link tree, laid out as <tracker>/<entry>.
	LinksRoot string
	// IncompleteRoot is where partial downloads live. Often empty: the
	// directory belongs to the torrent client's container and is not
	// necessarily mounted here.
	IncompleteRoot string
	// LibraryRoots are the trees the *arrs import into. Used only to bound the
	// library check; the paths themselves come from the database.
	LibraryRoots []string
	// Categories limits the download tree to what torrents actually manage.
	//
	// Load-bearing. The same tree holds soulseek/, rips/ and youtube/, which no
	// torrent ever claimed and which "unclaimed" therefore says nothing about —
	// treating them as leftovers would propose deleting the entire Soulseek
	// library.
	Categories []string
}

// Scanner finds leftovers.
type Scanner struct {
	q *query.Query
	// client is the torrent client, read directly for the claimed set. Nil
	// falls back to the database — see heldPaths for why that is second best.
	client Holder
	logger *slog.Logger
	cfg    Config
}

// New builds a scanner. client may be nil, which falls back to the database.
func New(db *gorm.DB, client Holder, logger *slog.Logger, cfg Config) *Scanner {
	return &Scanner{q: query.Use(db), client: client, logger: logger, cfg: cfg}
}

// ClassStats is what one class found.
type ClassStats struct {
	Count int
	Bytes int64
	// Files is the findings themselves, capped at maxReported.
	Files []Finding `json:",omitempty"`
	// Truncated says Files is a prefix of Count.
	Truncated bool `json:",omitempty"`
	// Deleted is how many entries were removed. Zero on a dry run, and zero
	// for the inverse classes, which have nothing on disk to remove.
	Deleted int `json:",omitempty"`
	// DeletedBytes is what those entries reported occupying. NOT what was
	// reclaimed: these trees are largely hardlinks, so the space returned is
	// whatever had no other link. See dirSize.
	DeletedBytes int64 `json:",omitempty"`
	// Spared is entries a torrent claimed between the scan and the delete. A
	// non-zero value means the walk raced a new torrent, which is the window
	// the re-check exists to close.
	Spared int `json:",omitempty"`
	// Failed is deletions the filesystem refused.
	Failed int `json:",omitempty"`
	// all is every finding, uncapped, and is what deletion iterates.
	//
	// Files is a REPORTING sample bounded by maxReported; deleting from it
	// would quietly stop at the cap and leave the remainder while claiming the
	// class was handled. The two must not be the same list.
	all []Finding
}

// maxReported bounds each class's list in a result.
const maxReported = 200

// Stats reports a whole sweep.
type Stats struct {
	DryRun bool
	Took   time.Duration
	// ByClass is keyed by Class. A class that found nothing is still present
	// with a zero count, so a missing key means the check did not run — which
	// is a different thing and should not read as "clean".
	ByClass map[Class]*ClassStats
	// Skipped names checks that could not run, with why. An unmounted
	// incomplete directory is the ordinary case.
	Skipped map[string]string `json:",omitempty"`
}

// deletableClasses are the findings that name something on disk to remove.
//
// The other two classes are the inverse — a row claiming a file that is not
// there — so there is nothing to delete for them. They are diagnostics: a
// library row pointing at a missing file means the *arr ingest is behind or the
// file was replaced, and a torrent whose data has vanished means the client is
// seeding nothing. Acting on either would mean writing to the database to hide
// a problem rather than fixing it.
var deletableClasses = []Class{
	ClassDownloadData,
	ClassCrossSeedLinks,
	ClassIncomplete,
}

// Run finds everything stranded and, unless dryRun, deletes what it can.
//
// Deletes only the disk-side classes — see deletableClasses. Dry run is the
// default at every layer above this, because the evidence for removing an entry
// is the absence of a claim, and absence is exactly what a stale or partial
// read produces. heldPaths reads the client directly for that reason.
func (s *Scanner) Run(ctx context.Context, dryRun bool) (Stats, error) {
	st := Stats{
		DryRun:  dryRun,
		ByClass: map[Class]*ClassStats{},
		Skipped: map[string]string{},
	}
	started := time.Now()

	held, err := s.heldPaths(ctx)
	if err != nil {
		return st, err
	}

	// Disk-side: something exists that nothing claims.
	if err := s.scanUnclaimed(ClassDownloadData, s.cfg.CompleteRoot, held, s.cfg.Categories, &st); err != nil {
		return st, err
	}
	if err := s.scanUnclaimed(ClassCrossSeedLinks, s.cfg.LinksRoot, held, nil, &st); err != nil {
		return st, err
	}
	if err := s.scanUnclaimed(ClassIncomplete, s.cfg.IncompleteRoot, held, nil, &st); err != nil {
		return st, err
	}

	// Inverse: something is claimed that does not exist.
	if err := s.scanMissingTorrentData(ctx, held, &st); err != nil {
		return st, err
	}
	if err := s.scanMissingLibraryFiles(ctx, &st); err != nil {
		return st, err
	}

	if !dryRun {
		if err := s.remove(ctx, &st); err != nil {
			return st, err
		}
	}

	st.Took = time.Since(started)
	return st, nil
}

// remove deletes the disk-side findings.
//
// Re-checks each entry immediately before removing it. The findings were
// gathered from a directory walk that may have taken minutes over NFS, and a
// torrent added since then would have its data here while the list still says
// nothing claims it — so the claim set is read again and every path tested
// against the fresh one. Belt and braces over a walk that is not atomic.
//
// A failure on one entry does not stop the rest: they are independent, and
// aborting would make the run's effect depend on which path happened to sort
// first.
func (s *Scanner) remove(ctx context.Context, st *Stats) error {
	held, err := s.heldPaths(ctx)
	if err != nil {
		return fmt.Errorf("reap: re-read claims before deleting: %w", err)
	}

	for _, class := range deletableClasses {
		cs, ok := st.ByClass[class]
		if !ok || cs.Count == 0 {
			continue
		}
		root := s.rootFor(class)
		claimed := claimedEntries(root, held)

		for _, f := range cs.all {
			if _, stillHeld := claimed[f.Path]; stillHeld {
				// Claimed between the walk and now. Reported rather than
				// silently skipped: it means the window is real and worth
				// knowing about.
				cs.Spared++
				s.logger.InfoContext(ctx, "reap: entry claimed since the scan, not deleted",
					"class", class, "path", f.Path)
				continue
			}
			if err := os.RemoveAll(f.Path); err != nil {
				cs.Failed++
				s.logger.ErrorContext(ctx, "reap: delete failed",
					"class", class, "path", f.Path, "err", err)
				continue
			}
			cs.Deleted++
			cs.DeletedBytes += f.Bytes
			s.logger.InfoContext(ctx, "reap: deleted",
				"class", class, "path", f.Path, "bytes", f.Bytes)
		}
	}
	return nil
}

// rootFor is the tree a class was walked from, needed to fold a path back to
// its entry when re-checking claims.
func (s *Scanner) rootFor(class Class) string {
	switch class {
	case ClassDownloadData:
		return s.cfg.CompleteRoot
	case ClassCrossSeedLinks:
		return s.cfg.LinksRoot
	case ClassIncomplete:
		return s.cfg.IncompleteRoot
	default:
		return ""
	}
}

// Holder reports what the torrent client currently holds.
//
// An interface so the scanner can be driven without a client in tests, and so
// the fallback to the database stays explicit rather than implicit.
type Holder interface {
	Torrents(ctx context.Context) ([]qbittorrent.Torrent, error)
}

// heldPaths is every path a live torrent reports.
//
// READ FROM THE CLIENT, not from client_torrents. The claimed set is the one
// input where staleness causes deletion, and the database is a reconcile
// snapshot: /reconcile answers 202 and runs asynchronously, so a sweep firing
// during a pass reads a table mid-write. That happened — a sweep ran 2m39s into
// an 8m44s reconcile and reported four cross-seed injections from twenty
// minutes earlier as abandoned, plus three freshly completed downloads whose
// rows still carried their pre-move incomplete paths.
//
// A torrent added a second ago is exactly the one most at risk from a stale
// read, because it is the one the snapshot cannot know about.
//
// Falls back to the database when no client is configured. Both save_path and
// content_path count as a claim: a torrent still fetching metadata has no
// content_path yet, and its reserved directory must not read as unclaimed.
func (s *Scanner) heldPaths(ctx context.Context) ([]string, error) {
	if s.client != nil {
		torrents, err := s.client.Torrents(ctx)
		if err != nil {
			return nil, fmt.Errorf("reap: held paths from client: %w", err)
		}
		paths := make([]string, 0, len(torrents)*2)
		for _, t := range torrents {
			if t.ContentPath != "" {
				paths = append(paths, t.ContentPath)
			}
			if t.SavePath != "" {
				paths = append(paths, t.SavePath)
			}
		}
		return paths, nil
	}

	ct := s.q.ClientTorrent
	var paths []string
	err := ct.WithContext(ctx).
		Distinct(ct.ContentPath).
		Where(ct.RemovedAt.IsNull(), ct.ContentPath.IsNotNull()).
		Pluck(ct.ContentPath, &paths)
	if err != nil {
		return nil, fmt.Errorf("reap: held paths: %w", err)
	}
	return paths, nil
}

// scanUnclaimed walks one tree and reports entries nothing holds.
//
// categories, when non-nil, limits which first-level directories are considered
// at all. See Config.Categories: a tree can hold things torrents never managed,
// and those are not leftovers however unclaimed they look.
func (s *Scanner) scanUnclaimed(class Class, root string, held, categories []string, st *Stats) error {
	if root == "" {
		st.Skipped[string(class)] = "no root configured"
		return nil
	}
	if _, err := os.Stat(root); err != nil {
		// Not an error: the directory belongs to another container and may
		// simply not be mounted here. Recorded so an empty result is not read
		// as a clean one.
		st.Skipped[string(class)] = fmt.Sprintf("%s not readable: %v", root, err)
		return nil
	}

	claimed := claimedEntries(root, held)
	cs := &ClassStats{}
	st.ByClass[class] = cs

	cats, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("reap: read %s: %w", root, err)
	}
	for _, cat := range cats {
		if !cat.IsDir() {
			continue
		}
		if categories != nil && !slices.Contains(categories, cat.Name()) {
			continue
		}

		catPath := path.Join(root, cat.Name())
		entries, err := os.ReadDir(catPath)
		if err != nil {
			return fmt.Errorf("reap: read %s: %w", catPath, err)
		}
		for _, e := range entries {
			entry := path.Join(catPath, e.Name())
			if _, ok := claimed[entry]; ok {
				continue
			}
			size := dirSize(entry)
			cs.Count++
			cs.Bytes += size
			found := Finding{
				Class:    class,
				Path:     entry,
				Bytes:    size,
				Category: cat.Name(),
			}
			// Recorded in full for deletion; Files is only the sample a human
			// reads, and stopping deletion at that cap would leave entries
			// behind while reporting the class as handled.
			cs.all = append(cs.all, found)
			if len(cs.Files) < maxReported {
				cs.Files = append(cs.Files, found)
			} else {
				cs.Truncated = true
			}
		}
	}
	return nil
}

// scanMissingTorrentData reports torrents whose files are gone.
//
// A torrent in this state seeds nothing: the client has the metadata, the peers
// ask for pieces it cannot read, and no amount of waiting fixes it.
func (s *Scanner) scanMissingTorrentData(ctx context.Context, held []string, st *Stats) error {
	cs := &ClassStats{}
	st.ByClass[ClassTorrentDataMissing] = cs
	var skipped int

	// Bounded to the roots this process can actually see, the way the library
	// check already is. Without it, every torrent whose content_path lies
	// outside managarr's mounts reports as missing: a torrent still
	// downloading has its path under the client's incomplete directory, which
	// belongs to the qBittorrent container and is deliberately NOT mounted
	// here — nine live downloads were reported as data loss on that basis.
	roots := s.visibleRoots()

	for _, p := range held {
		if !withinRoots(roots, p) {
			// Not visible from here. Recorded as a skip rather than silently
			// dropped: an unrun check and a clean one must not look the same.
			skipped++
			continue
		}
		if _, err := os.Stat(p); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			// Unreadable for some other reason — a permissions problem or a
			// stale NFS handle. Not evidence the data is gone, so not reported
			// as such.
			continue
		}
		cs.Count++
		if len(cs.Files) < maxReported {
			cs.Files = append(cs.Files, Finding{
				Class: ClassTorrentDataMissing,
				Path:  p,
			})
		} else {
			cs.Truncated = true
		}
	}

	if skipped > 0 {
		st.Skipped[string(ClassTorrentDataMissing)] = fmt.Sprintf(
			"%d torrent paths lie outside this container's mounts and were not checked", skipped)
	}
	return nil
}

// visibleRoots is every tree this process can stat.
//
// The claim "this file is gone" is only meaningful where the file would have
// been visible in the first place.
func (s *Scanner) visibleRoots() []string {
	roots := make([]string, 0, len(s.cfg.LibraryRoots)+3)
	roots = append(roots, s.cfg.LibraryRoots...)
	for _, r := range []string{s.cfg.CompleteRoot, s.cfg.LinksRoot, s.cfg.IncompleteRoot} {
		if r != "" {
			roots = append(roots, r)
		}
	}
	return roots
}

// scanMissingLibraryFiles reports *arr rows whose file is not on disk.
//
// Bounded to LibraryRoots so a row pointing somewhere unmounted is skipped
// rather than reported as missing — the file may be perfectly present on a
// path this process cannot see.
func (s *Scanner) scanMissingLibraryFiles(ctx context.Context, st *Stats) error {
	if s.q == nil {
		// No database, so the library's own view is unavailable. Recorded as a
		// skip rather than an empty result: the two must not look alike.
		st.Skipped[string(ClassLibraryFileMissing)] = "no database configured"
		return nil
	}

	cs := &ClassStats{}
	st.ByClass[ClassLibraryFileMissing] = cs

	lf := s.q.LibraryFile
	rows, err := lf.WithContext(ctx).
		Select(lf.Path, lf.ServiceName).
		Where(lf.RemovedAt.IsNull()).
		Find()
	if err != nil {
		return fmt.Errorf("reap: library files: %w", err)
	}

	for _, r := range rows {
		if !underAny(s.cfg.LibraryRoots, r.Path) {
			continue
		}
		if _, err := os.Stat(r.Path); err == nil || !os.IsNotExist(err) {
			continue
		}
		service := ""
		if r.ServiceName != nil {
			service = *r.ServiceName
		}
		cs.Count++
		if len(cs.Files) < maxReported {
			cs.Files = append(cs.Files, Finding{
				Class:  ClassLibraryFileMissing,
				Path:   r.Path,
				Detail: service,
			})
		} else {
			cs.Truncated = true
		}
	}
	return nil
}

// underAny reports whether p sits under one of the roots.
func underAny(roots []string, p string) bool {
	for _, root := range roots {
		if _, ok := entryUnder(root, p); ok {
			return true
		}
	}
	return false
}

// dirSize totals a path, following no symlinks and crossing no errors.
//
// Hardlinks are counted at face value rather than deduplicated. That overstates
// what deleting would reclaim wherever a library import or a cross-seed inject
// still holds the same bytes — which is stated in the result rather than
// corrected for, because correcting for it means counting links, and counting
// links is how the tools this replaced got the question wrong.
func dirSize(p string) int64 {
	info, err := os.Lstat(p)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		return info.Size()
	}

	var total int64
	_ = fs.WalkDir(os.DirFS(p), ".", func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			// Skip rather than abort: an unreadable subdirectory makes the
			// total an underestimate, which is the safe direction for a number
			// that argues for deleting something.
			return nil //nolint:nilerr // a partial total beats no total
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		total += fi.Size()
		return nil
	})
	return total
}
