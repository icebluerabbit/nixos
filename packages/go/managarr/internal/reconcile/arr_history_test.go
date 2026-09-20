package reconcile

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"homelab/clients/arr"
	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
	"homelab/managarr/internal/store"
)

// testArr opens a real database, skipping when none is configured.
//
// markRemovals is raw SQL against a VALUES join. Nothing about it can be
// checked without executing it: a wrong column name, a missing cast or a
// mis-scoped CASE are all runtime failures on a production pass.
func testArr(t *testing.T) (*Arr, *dao.DAO, context.Context) {
	t.Helper()
	dsn := os.Getenv("MANAGARR_TEST_DSN")
	if dsn == "" {
		t.Skip("MANAGARR_TEST_DSN not set")
	}
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	db, err := store.OpenGorm(ctx, dsn, logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AutoMigrate(db, logger); err != nil {
		t.Fatal(err)
	}
	d := dao.New(db, logger)
	return &Arr{dao: d, lookups: dao.NewLookups(d), logger: logger,
		serviceName: "test-arr"}, d, ctx
}

// newFile inserts a library file at a unique path and returns it.
func newFile(t *testing.T, d *dao.DAO, ctx context.Context) model.LibraryFile {
	t.Helper()
	f := model.LibraryFile{Path: "/media/test/" + model.NewID().String() + ".mkv"}
	if err := d.DB().WithContext(ctx).Create(&f).Error; err != nil {
		t.Fatal(err)
	}
	return f
}

func reload(t *testing.T, d *dao.DAO, ctx context.Context, id model.ID) model.LibraryFile {
	t.Helper()
	var got model.LibraryFile
	if err := d.DB().WithContext(ctx).First(&got, "id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	return got
}

// TestMarkRemovalsRecordsAllThreeReasons is the point of the whole change.
//
// A file the *arr no longer tracks just stops appearing in its file list, so a
// snapshot cannot tell "upgraded" from "deleted by hand" from "vanished off the
// disk" — it cannot even tell the file was ever there. History says all three
// outright and keeps them forever, which is what makes this backfillable.
func TestMarkRemovalsRecordsAllThreeReasons(t *testing.T) {
	a, d, ctx := testArr(t)

	upgraded := newFile(t, d, ctx)
	manual := newFile(t, d, ctx)
	missing := newFile(t, d, ctx)
	at := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

	n, err := a.markRemovals(ctx, []removal{
		{Path: upgraded.Path, At: at, Reason: "Upgrade"},
		{Path: manual.Path, At: at, Reason: "Manual"},
		{Path: missing.Path, At: at, Reason: "MissingFromDisk"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("marked %d rows, want 3", n)
	}

	for _, c := range []struct {
		file   model.LibraryFile
		reason string
	}{
		{upgraded, "upgrade"}, {manual, "manual"}, {missing, "missingfromdisk"},
	} {
		got := reload(t, d, ctx, c.file.ID)
		if got.RemovedAt == nil || !got.RemovedAt.Equal(at) {
			t.Errorf("%s: removed_at = %v", c.reason, got.RemovedAt)
		}
		// Lowercased so a query does not have to know each app's casing.
		if got.RemovalReason == nil || *got.RemovalReason != c.reason {
			t.Errorf("removal_reason = %v, want %q", got.RemovalReason, c.reason)
		}
	}
}

// TestMarkRemovalsSupersedesOnlyOnUpgrade is the distinction that matters.
//
// Removed and superseded are not the same claim. Only an upgrade says something
// arrived to take the file's place; a manual delete says the opposite.
func TestMarkRemovalsSupersedesOnlyOnUpgrade(t *testing.T) {
	a, d, ctx := testArr(t)

	upgraded := newFile(t, d, ctx)
	manual := newFile(t, d, ctx)
	at := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

	if _, err := a.markRemovals(ctx, []removal{
		{Path: upgraded.Path, At: at, Reason: "Upgrade"},
		{Path: manual.Path, At: at, Reason: "Manual"},
	}); err != nil {
		t.Fatal(err)
	}

	if got := reload(t, d, ctx, upgraded.ID); got.SupersededAt == nil {
		t.Error("an upgrade did not set superseded_at")
	}
	if got := reload(t, d, ctx, manual.ID); got.SupersededAt != nil {
		t.Errorf("a manual delete set superseded_at = %v", got.SupersededAt)
	}
}

// TestMarkRemovalsKeepsTheEarliestSupersede guards against a reused path.
//
// The *arr writes to the same path repeatedly as a file is upgraded again and
// again. The first replacement is the true one; a later delete at that path is
// about a different file.
func TestMarkRemovalsKeepsTheEarliestSupersede(t *testing.T) {
	a, d, ctx := testArr(t)

	f := newFile(t, d, ctx)
	first := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)

	if _, err := a.markRemovals(ctx, []removal{
		{Path: f.Path, At: first, Reason: "Upgrade"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.markRemovals(ctx, []removal{
		{Path: f.Path, At: later, Reason: "Upgrade"},
	}); err != nil {
		t.Fatal(err)
	}

	got := reload(t, d, ctx, f.ID)
	if got.SupersededAt == nil || !got.SupersededAt.Equal(first) {
		t.Errorf("superseded_at = %v, want the earliest %v", got.SupersededAt, first)
	}
	// removed_at is the opposite: it is "as of the last look", so the newer
	// value wins.
	if got.RemovedAt == nil || !got.RemovedAt.Equal(later) {
		t.Errorf("removed_at = %v, want the latest %v", got.RemovedAt, later)
	}
}

// TestMarkRemovalsIgnoresUnknownPaths covers old history.
//
// History runs back further than the library does — a file deleted before this
// service existed has no row to mark, and that is not an error.
func TestMarkRemovalsIgnoresUnknownPaths(t *testing.T) {
	a, _, ctx := testArr(t)

	n, err := a.markRemovals(ctx, []removal{
		{Path: "/media/never/existed.mkv", At: time.Now().UTC(), Reason: "Upgrade"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("marked %d rows for a path with no file", n)
	}
}

// TestMarkRemovalsChunks proves the VALUES join survives a batch larger than
// one chunk.
func TestMarkRemovalsChunks(t *testing.T) {
	a, d, ctx := testArr(t)

	const n = importChunk + 3
	removals := make([]removal, 0, n)
	ids := make([]model.ID, 0, n)
	at := time.Now().UTC().Truncate(time.Microsecond)

	for range n {
		f := newFile(t, d, ctx)
		ids = append(ids, f.ID)
		removals = append(removals, removal{Path: f.Path, At: at, Reason: "Upgrade"})
	}

	got, err := a.markRemovals(ctx, removals)
	if err != nil {
		t.Fatal(err)
	}
	if got != n {
		t.Fatalf("marked %d rows, want %d", got, n)
	}
	if f := reload(t, d, ctx, ids[n-1]); f.RemovedAt == nil {
		t.Error("the last row of the final chunk was not marked")
	}
}

func TestMarkRemovalsEmptyIsANoOp(t *testing.T) {
	a, _, ctx := testArr(t)
	if n, err := a.markRemovals(ctx, nil); err != nil || n != 0 {
		t.Errorf("got %d, %v", n, err)
	}
}

// renameRecord builds a rename history record the way all three apps send one.
func renameRecord(from, to string) arr.HistoryRecord {
	return arr.HistoryRecord{
		EventType:   "episodeFileRenamed",
		SourceTitle: from,
		Data: map[string]string{
			arr.DataSourcePath: from,
			arr.DataPath:       to,
		},
	}
}

// TestApplyRenamesMovesTheRowRatherThanDuplicatingIt is the actual defect.
//
// path is the unique key and the join target for import provenance and for
// Bazarr's subtitles. Ignoring a rename does not merely lose a record — the next
// file snapshot inserts a second row at the new path, the old row keeps the
// infohash, and the provenance detaches with both rows looking valid.
func TestApplyRenamesMovesTheRowRatherThanDuplicatingIt(t *testing.T) {
	a, d, ctx := testArr(t)

	f := newFile(t, d, ctx)
	// The full id: a v7 UUID encodes its timestamp in the LEADING bits, so a
	// prefix slice is identical for every id minted in the same span.
	hash := strings.ReplaceAll(model.NewID().String(), "-", "")
	if err := d.DB().WithContext(ctx).Create(&model.Torrent{Infohash: hash}).Error; err != nil {
		t.Fatal(err)
	}
	if err := d.DB().WithContext(ctx).Model(&model.LibraryFile{}).
		Where("id = ?", f.ID).Update("torrent_infohash", hash).Error; err != nil {
		t.Fatal(err)
	}

	to := f.Path + ".renamed.mkv"
	n, err := a.applyRenames(ctx, []arr.HistoryRecord{renameRecord(f.Path, to)})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("renamed %d rows, want 1", n)
	}

	got := reload(t, d, ctx, f.ID)
	if got.Path != to {
		t.Errorf("path = %q, want %q", got.Path, to)
	}
	// The whole point: the row moved, so its provenance moved with it.
	if got.TorrentInfohash == nil || *got.TorrentInfohash != hash {
		t.Errorf("infohash = %v; provenance did not follow the rename", got.TorrentInfohash)
	}

	var count int64
	if err := d.DB().WithContext(ctx).Model(&model.LibraryFile{}).
		Where("path = ?", f.Path).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("%d rows left behind at the old path", count)
	}
}

// TestApplyRenamesFollowsAChain checks the ordering.
//
// History is walked oldest-first, so a file renamed twice must be followed a to
// b to c. Applying them in any other order strands the row at a path the *arr
// no longer uses.
func TestApplyRenamesFollowsAChain(t *testing.T) {
	a, d, ctx := testArr(t)

	f := newFile(t, d, ctx)
	b := f.Path + ".b"
	c := f.Path + ".c"

	n, err := a.applyRenames(ctx, []arr.HistoryRecord{
		renameRecord(f.Path, b),
		renameRecord(b, c),
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("applied %d renames, want 2", n)
	}
	if got := reload(t, d, ctx, f.ID); got.Path != c {
		t.Errorf("path = %q, want the last hop %q", got.Path, c)
	}
}

// TestApplyRenamesSkipsAnOccupiedTarget is the non-destructive guard.
//
// On a database where the snapshot already created the second row, forcing the
// rename would violate the unique key. Merging the two means choosing which
// row's columns survive, and guessing wrong silently reattributes a file.
func TestApplyRenamesSkipsAnOccupiedTarget(t *testing.T) {
	a, d, ctx := testArr(t)

	from := newFile(t, d, ctx)
	to := newFile(t, d, ctx)

	n, err := a.applyRenames(ctx, []arr.HistoryRecord{renameRecord(from.Path, to.Path)})
	if err != nil {
		t.Fatalf("an occupied target must not be an error: %v", err)
	}
	if n != 0 {
		t.Errorf("applied %d renames onto an occupied path", n)
	}
	if got := reload(t, d, ctx, from.ID); got.Path != from.Path {
		t.Errorf("source row moved to %q", got.Path)
	}
}

// TestApplyRenamesIgnoresUnusableRecords covers the shapes that say nothing.
func TestApplyRenamesIgnoresUnusableRecords(t *testing.T) {
	a, _, ctx := testArr(t)

	same := "/media/test/same.mkv"
	records := []arr.HistoryRecord{
		// Not a rename at all.
		{EventType: "grabbed", Data: map[string]string{}},
		// No destination.
		{EventType: "episodeFileRenamed", SourceTitle: "/a", Data: map[string]string{}},
		// No source.
		{EventType: "episodeFileRenamed", Data: map[string]string{arr.DataPath: "/b"}},
		// A no-op rename.
		renameRecord(same, same),
	}
	n, err := a.applyRenames(ctx, records)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("applied %d renames from unusable records", n)
	}
}

// TestApplyRenamesFallsBackToSourceTitle covers builds that send only it.
//
// Every app sets sourceTitle to the original path on a rename, so it is the
// safety net when the data map is not what this expects.
func TestApplyRenamesFallsBackToSourceTitle(t *testing.T) {
	a, d, ctx := testArr(t)

	f := newFile(t, d, ctx)
	to := f.Path + ".renamed"

	n, err := a.applyRenames(ctx, []arr.HistoryRecord{{
		EventType:   "movieFileRenamed",
		SourceTitle: f.Path,
		Data:        map[string]string{arr.DataPath: to},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("renamed %d rows, want 1", n)
	}
	if got := reload(t, d, ctx, f.ID); got.Path != to {
		t.Errorf("path = %q", got.Path)
	}
}

// TestFailureKindKeepsTheThreeOutcomesApart is why the column exists.
//
// "The import broke", "I decided not to bother" and "half the album landed" are
// three different statements about whose fault it was. Folding them together
// would make them the same answer.
func TestFailureKindKeepsTheThreeOutcomesApart(t *testing.T) {
	cases := map[arr.EventKind]string{
		arr.EventFailed:           model.ImportFailed,
		arr.EventIgnored:          model.ImportIgnored,
		arr.EventImportIncomplete: model.ImportIncomplete,
	}
	seen := map[string]bool{}
	for ev, want := range cases {
		got := failureKind(ev)
		if got != want {
			t.Errorf("failureKind(%q) = %q, want %q", ev, got, want)
		}
		if seen[got] {
			t.Errorf("%q collided with another outcome", got)
		}
		seen[got] = true
	}

	// The column is NOT NULL, so an event routed here by mistake must still
	// produce a writable row rather than an empty string.
	for _, ev := range []arr.EventKind{arr.EventGrabbed, arr.EventUnknown, ""} {
		if got := failureKind(ev); got != model.ImportFailed {
			t.Errorf("failureKind(%q) = %q, want a safe default", ev, got)
		}
	}
}

// TestFirstNonEmptyPicksTheExplanation covers the three keys the reason hides
// behind.
//
// Each outcome puts its text somewhere different — reason on a failure, message
// on an ignore, statusMessages on a partial import — and an empty reason is the
// one thing that makes any of these rows useless.
func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "third"); got != "third" {
		t.Errorf("got %q", got)
	}
	if got := firstNonEmpty("first", "second"); got != "first" {
		t.Errorf("got %q, want the earliest non-empty", got)
	}
	if got := firstNonEmpty("", "", ""); got != "" {
		t.Errorf("got %q", got)
	}
	if got := firstNonEmpty(); got != "" {
		t.Errorf("got %q", got)
	}
}

// TestFailureRowRecordsEachOutcome walks a record of each kind through the
// builder, against the real column set.
func TestFailureRowRecordsEachOutcome(t *testing.T) {
	a, _, ctx := testArr(t)

	cases := []struct {
		name   string
		record arr.HistoryRecord
		kind   string
		reason string
	}{
		{
			name: "failed",
			record: arr.HistoryRecord{
				EventType:   "downloadFailed",
				SourceTitle: "Some.Release-GRP",
				Data:        map[string]string{arr.DataReason: "Unpack failed"},
			},
			kind:   model.ImportFailed,
			reason: "Unpack failed",
		},
		{
			name: "ignored",
			record: arr.HistoryRecord{
				EventType:   "downloadIgnored",
				SourceTitle: "Some.Release-GRP",
				Data:        map[string]string{arr.DataMessage: "Manually ignored"},
			},
			kind:   model.ImportIgnored,
			reason: "Manually ignored",
		},
		{
			name: "incomplete",
			record: arr.HistoryRecord{
				EventType:   "albumImportIncomplete",
				SourceTitle: "Some.Album-GRP",
				Data: map[string]string{
					arr.DataStatusMessages: `[{"title":"One or more tracks expected in this release were not imported"}]`,
				},
			},
			kind:   model.ImportIncomplete,
			reason: `[{"title":"One or more tracks expected in this release were not imported"}]`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.record.ID = int64(len(c.name))
			row, err := a.failureRow(ctx, c.record, nil, failureKind(c.record.Kind()))
			if err != nil {
				t.Fatal(err)
			}
			if row.Kind != c.kind {
				t.Errorf("kind = %q, want %q", row.Kind, c.kind)
			}
			if deref(row.Reason) != c.reason {
				t.Errorf("reason = %q, want %q", deref(row.Reason), c.reason)
			}
			// sourceTitle is the fallback path when no dropped path is given,
			// and it is what names the release that failed.
			if deref(row.SourcePath) != c.record.SourceTitle {
				t.Errorf("source path = %q", deref(row.SourcePath))
			}
		})
	}
}

// uniqueService creates a service row and returns its name.
func uniqueService(t *testing.T, d *dao.DAO, ctx context.Context) string {
	t.Helper()
	name := "svc-" + model.NewID().String()
	if err := d.DB().WithContext(ctx).Create(&model.Service{
		Name: name, Kind: "test",
	}).Error; err != nil {
		t.Fatal(err)
	}
	return name
}

// nowUTC is a stable timestamp for rows whose time is not under test.
func nowUTC() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }
