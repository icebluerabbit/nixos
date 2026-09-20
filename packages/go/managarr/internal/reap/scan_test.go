package reap

import (
	"context"
	"io"
	"log/slog"

	"homelab/clients/qbittorrent"

	"os"
	"path/filepath"
	"testing"
)

// mkTree builds a download tree on disk and returns its root.
func mkTree(t *testing.T, entries map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range entries {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// scan runs only the unclaimed walk, which is the part that needs no database.
func scan(t *testing.T, root string, held, cats []string) *ClassStats {
	t.Helper()
	s := &Scanner{cfg: Config{}}
	st := &Stats{ByClass: map[Class]*ClassStats{}, Skipped: map[string]string{}}
	if err := s.scanUnclaimed(ClassDownloadData, root, held, cats, st); err != nil {
		t.Fatal(err)
	}
	return st.ByClass[ClassDownloadData]
}

// The failure this package exists to avoid, end to end on a real tree: a
// torrent holding one file inside a folder must not make the folder look
// unclaimed.
func TestSeedingFolderIsNotReaped(t *testing.T) {
	root := mkTree(t, map[string]string{
		"books/An Author - A Book/book.epub": "content",
	})
	held := []string{filepath.Join(root, "books", "An Author - A Book", "book.epub")}

	cs := scan(t, root, held, []string{"books"})
	if cs.Count != 0 {
		t.Fatalf("REAPED A SEEDING TORRENT: %+v", cs.Files)
	}
}

// The base case: an entry no reported path touches.
func TestUnclaimedEntryIsFound(t *testing.T) {
	root := mkTree(t, map[string]string{
		"tv/Live.S01E01.mkv":      "held",
		"tv/Abandoned.S01E02.mkv": "orphan",
	})
	held := []string{filepath.Join(root, "tv", "Live.S01E01.mkv")}

	cs := scan(t, root, held, []string{"tv"})
	if cs.Count != 1 {
		t.Fatalf("count = %d, want 1: %+v", cs.Count, cs.Files)
	}
	if got := filepath.Base(cs.Files[0].Path); got != "Abandoned.S01E02.mkv" {
		t.Errorf("found %q", got)
	}
	if cs.Files[0].Category != "tv" {
		t.Errorf("category = %q", cs.Files[0].Category)
	}
	if cs.Bytes != int64(len("orphan")) {
		t.Errorf("bytes = %d, want %d", cs.Bytes, len("orphan"))
	}
}

// Categories outside the configured set are not examined at all.
//
// The tree holds soulseek/, rips/ and youtube/ alongside the torrent
// categories. No torrent ever claimed those, so "unclaimed" says nothing about
// them — and reaping on it would propose deleting the entire Soulseek library.
func TestUnmanagedCategoriesAreNeverConsidered(t *testing.T) {
	root := mkTree(t, map[string]string{
		"soulseek/Some Album/track.flac": "music",
		"rips/Another Album/track.flac":  "music",
		"youtube/Some Video.mkv":         "video",
		"tv/Abandoned.mkv":               "orphan",
	})

	cs := scan(t, root, nil, []string{"tv", "movies", "books"})
	if cs.Count != 1 {
		t.Fatalf("count = %d, want only the tv entry: %+v", cs.Count, cs.Files)
	}
	for _, f := range cs.Files {
		if f.Category != "tv" {
			t.Fatalf("PROPOSED DELETING %s DATA: %q", f.Category, f.Path)
		}
	}
}

// A nil category list means every directory, which is what the cross-seed link
// tree needs — its first level is a tracker name, not a category.
func TestNilCategoriesConsidersEverything(t *testing.T) {
	root := mkTree(t, map[string]string{
		"YU-Scene/Show.S01E08.mkv": "orphan",
		"SeedPool/Show.S01E09.mkv": "orphan",
	})

	cs := scan(t, root, nil, nil)
	if cs.Count != 2 {
		t.Fatalf("count = %d, want 2: %+v", cs.Count, cs.Files)
	}
}

// A multi-file torrent is claimed by any path inside it, however deep.
func TestDeepPathClaimsItsEntry(t *testing.T) {
	root := mkTree(t, map[string]string{
		"movies/Film.2024/Film.mkv":    "video",
		"movies/Film.2024/Subs/en.srt": "subs",
	})
	held := []string{filepath.Join(root, "movies", "Film.2024", "Subs", "en.srt")}

	cs := scan(t, root, held, []string{"movies"})
	if cs.Count != 0 {
		t.Fatalf("claimed entry was reaped: %+v", cs.Files)
	}
}

// A missing root is recorded as skipped rather than reported as clean. An empty
// result and an unrun check must not look the same.
func TestMissingRootIsSkippedNotClean(t *testing.T) {
	s := &Scanner{cfg: Config{}}
	st := &Stats{ByClass: map[Class]*ClassStats{}, Skipped: map[string]string{}}
	if err := s.scanUnclaimed(ClassIncomplete, "/nonexistent/path", nil, nil, st); err != nil {
		t.Fatal(err)
	}
	if _, ok := st.ByClass[ClassIncomplete]; ok {
		t.Error("an unreadable root produced a class result, which reads as clean")
	}
	if st.Skipped[string(ClassIncomplete)] == "" {
		t.Error("skip was not recorded")
	}
}

// An unconfigured root is skipped the same way.
func TestEmptyRootIsSkipped(t *testing.T) {
	s := &Scanner{cfg: Config{}}
	st := &Stats{ByClass: map[Class]*ClassStats{}, Skipped: map[string]string{}}
	if err := s.scanUnclaimed(ClassIncomplete, "", nil, nil, st); err != nil {
		t.Fatal(err)
	}
	if st.Skipped[string(ClassIncomplete)] != "no root configured" {
		t.Errorf("skipped = %q", st.Skipped[string(ClassIncomplete)])
	}
}

// dirSize totals a folder rather than reporting the directory inode's own size.
func TestDirSizeTotalsContents(t *testing.T) {
	root := mkTree(t, map[string]string{
		"movies/Film/a.mkv":      "aaaa",
		"movies/Film/subs/b.srt": "bb",
	})
	if got := dirSize(filepath.Join(root, "movies", "Film")); got != 6 {
		t.Errorf("dirSize = %d, want 6", got)
	}
}

func TestDirSizeOfSingleFile(t *testing.T) {
	root := mkTree(t, map[string]string{"tv/one.mkv": "12345"})
	if got := dirSize(filepath.Join(root, "tv", "one.mkv")); got != 5 {
		t.Errorf("dirSize = %d, want 5", got)
	}
}

// A torrent path outside this container's mounts is SKIPPED, not reported gone.
//
// The failure this prevents: qBittorrent reports a still-downloading torrent's
// path under its own incomplete directory, which belongs to the client's
// container and is deliberately not mounted into managarr. Stat'ing it blind
// reported nine live downloads as data loss.
func TestUnmountedTorrentPathIsSkippedNotMissing(t *testing.T) {
	root := mkTree(t, map[string]string{"tv/Held.mkv": "x"})
	s := &Scanner{cfg: Config{CompleteRoot: root}}
	st := &Stats{ByClass: map[Class]*ClassStats{}, Skipped: map[string]string{}}

	held := []string{
		filepath.Join(root, "tv", "Held.mkv"),       // visible, present
		"/var/tmp/qbittorrent/xxx/Downloading.part", // outside every root
	}
	if err := s.scanMissingTorrentData(t.Context(), held, st); err != nil {
		t.Fatal(err)
	}

	cs := st.ByClass[ClassTorrentDataMissing]
	if cs.Count != 0 {
		t.Fatalf("REPORTED A LIVE DOWNLOAD AS MISSING: %+v", cs.Files)
	}
	if st.Skipped[string(ClassTorrentDataMissing)] == "" {
		t.Error("the unchecked path was not recorded as skipped")
	}
}

// A path inside a mounted root that genuinely is not there is still reported.
func TestVisibleMissingPathIsReported(t *testing.T) {
	root := mkTree(t, map[string]string{"tv/Present.mkv": "x"})
	s := &Scanner{cfg: Config{CompleteRoot: root}}
	st := &Stats{ByClass: map[Class]*ClassStats{}, Skipped: map[string]string{}}

	held := []string{filepath.Join(root, "tv", "Gone.mkv")}
	if err := s.scanMissingTorrentData(t.Context(), held, st); err != nil {
		t.Fatal(err)
	}
	if st.ByClass[ClassTorrentDataMissing].Count != 1 {
		t.Errorf("a genuinely absent file was not reported")
	}
}

func TestWithinRoots(t *testing.T) {
	roots := []string{"/mnt/media/library", "/mnt/media/complete"}
	cases := map[string]bool{
		"/mnt/media/library/tv/Show/ep.mkv": true,
		"/mnt/media/complete":               true,
		"/mnt/media/complete/tv/x.mkv":      true,
		"/var/tmp/qbittorrent/xxx/y":        false,
		// A sibling sharing a prefix must not match.
		"/mnt/media/library-old/tv/x.mkv": false,
		"/mnt/media/completed/x.mkv":      false,
	}
	for p, want := range cases {
		if got := withinRoots(roots, p); got != want {
			t.Errorf("withinRoots(%q) = %v, want %v", p, got, want)
		}
	}
}

// A dry run finds everything and removes nothing.
func TestDryRunDeletesNothing(t *testing.T) {
	root := mkTree(t, map[string]string{"tv/Abandoned.mkv": "orphan"})
	s := &Scanner{cfg: Config{CompleteRoot: root, Categories: []string{"tv"}}, logger: discardLogger(), client: &fakeHolder{}}

	st, err := s.Run(t.Context(), true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "tv", "Abandoned.mkv")); err != nil {
		t.Fatalf("A DRY RUN DELETED A FILE: %v", err)
	}
	if cs := st.ByClass[ClassDownloadData]; cs.Deleted != 0 {
		t.Errorf("deleted = %d on a dry run", cs.Deleted)
	}
}

// Applying removes the unclaimed entry and leaves the claimed one.
func TestApplyDeletesOnlyUnclaimed(t *testing.T) {
	root := mkTree(t, map[string]string{
		"tv/Held.mkv":      "keep",
		"tv/Abandoned.mkv": "go",
	})
	s := &Scanner{
		cfg:    Config{CompleteRoot: root, Categories: []string{"tv"}},
		logger: discardLogger(),
		client: &fakeHolder{paths: []string{filepath.Join(root, "tv", "Held.mkv")}},
	}

	st, err := s.Run(t.Context(), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "tv", "Held.mkv")); err != nil {
		t.Fatalf("DELETED A CLAIMED ENTRY: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tv", "Abandoned.mkv")); !os.IsNotExist(err) {
		t.Errorf("unclaimed entry survived: %v", err)
	}
	if cs := st.ByClass[ClassDownloadData]; cs.Deleted != 1 {
		t.Errorf("deleted = %d, want 1", cs.Deleted)
	}
}

// The re-check before deleting. The scan walks a tree that can take minutes
// over NFS; a torrent added in that window has its data here while the finding
// list still says nothing claims it.
func TestEntryClaimedDuringTheScanIsSpared(t *testing.T) {
	root := mkTree(t, map[string]string{"tv/JustAdded.mkv": "data"})
	entry := filepath.Join(root, "tv", "JustAdded.mkv")

	// Claims nothing on the first read (the scan) and the entry on the second
	// (the re-check), which is exactly the race.
	s := &Scanner{
		cfg:    Config{CompleteRoot: root, Categories: []string{"tv"}},
		logger: discardLogger(),
		client: &fakeHolder{afterFirst: []string{entry}},
	}

	st, err := s.Run(t.Context(), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(entry); err != nil {
		t.Fatalf("DELETED A TORRENT ADDED DURING THE SCAN: %v", err)
	}
	cs := st.ByClass[ClassDownloadData]
	if cs.Spared != 1 {
		t.Errorf("spared = %d, want 1", cs.Spared)
	}
	if cs.Deleted != 0 {
		t.Errorf("deleted = %d, want 0", cs.Deleted)
	}
}

// The inverse classes name rows, not files, so applying must not touch disk for
// them — there is nothing there to remove.
func TestInverseClassesAreNeverDeleted(t *testing.T) {
	root := mkTree(t, map[string]string{"tv/Present.mkv": "x"})
	s := &Scanner{cfg: Config{CompleteRoot: root, Categories: []string{"tv"}}, logger: discardLogger(), client: &fakeHolder{}}
	st := &Stats{ByClass: map[Class]*ClassStats{}, Skipped: map[string]string{}}

	// A missing path, which is what that class reports.
	held := []string{filepath.Join(root, "tv", "Gone.mkv")}
	if err := s.scanMissingTorrentData(t.Context(), held, st); err != nil {
		t.Fatal(err)
	}
	if err := s.remove(t.Context(), st); err != nil {
		t.Fatal(err)
	}
	if cs := st.ByClass[ClassTorrentDataMissing]; cs.Deleted != 0 {
		t.Errorf("deleted %d from an inverse class", cs.Deleted)
	}
	if _, err := os.Stat(filepath.Join(root, "tv", "Present.mkv")); err != nil {
		t.Errorf("an unrelated file was removed: %v", err)
	}
}

// fakeHolder reports claimed paths, optionally changing between calls so a
// scan-then-recheck race can be exercised.
type fakeHolder struct {
	paths      []string
	afterFirst []string
	calls      int
}

func (f *fakeHolder) Torrents(context.Context) ([]qbittorrent.Torrent, error) {
	f.calls++
	set := f.paths
	if f.calls > 1 && f.afterFirst != nil {
		set = f.afterFirst
	}
	out := make([]qbittorrent.Torrent, 0, len(set))
	for _, p := range set {
		out = append(out, qbittorrent.Torrent{ContentPath: p})
	}
	return out, nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
