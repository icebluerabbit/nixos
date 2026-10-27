package dao

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"homelab/managarr/internal/model"
	"homelab/managarr/internal/store"
)

// These run against a real PostgreSQL, not a mock.
//
// Everything worth testing here is behaviour Postgres owns: which index an
// ON CONFLICT arbitrates against, whether a guarded update fires, whether a
// partial index matches. A mock would assert that this package builds the SQL
// it was written to build — which is true by construction and has never been
// the bug. The bugs have all been the database disagreeing.
//
//	MANAGARR_TEST_DSN=postgres://managarr@host:5432/managarr_test?sslmode=disable go test ./...
const testDSNEnv = "MANAGARR_TEST_DSN"

// testDAO opens a connection and reconciles the schema, skipping when no
// database is configured.
func testDAO(t *testing.T) (*DAO, context.Context) {
	t.Helper()
	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Skipf("%s not set", testDSNEnv)
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
	return New(db, logger), ctx
}

// uniqueName keeps parallel and repeated runs from colliding, since these tests
// write into a shared database and never truncate it.
func uniqueName(prefix string) string {
	return prefix + "-" + model.NewID().String()
}

func TestUpsertInsertsThenUpdatesNamedColumns(t *testing.T) {
	d, ctx := testDAO(t)
	name := uniqueName("svc")

	first := model.Service{Name: name, Kind: "sonarr", Version: str("1.0")}
	if err := Upsert(ctx, d, []model.Service{first}, []string{"name"},
		[]string{"kind", "version"}); err != nil {
		t.Fatal(err)
	}

	// last_seen_at is deliberately NOT in the update list below, so it must
	// survive a second write that leaves it nil. This is the mechanism that
	// lets a snapshot writer and a webhook writer own different columns of one
	// row.
	seen := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	if err := d.DB().WithContext(ctx).Model(&model.Service{}).
		Where("name = ?", name).Update("last_seen_at", seen).Error; err != nil {
		t.Fatal(err)
	}

	second := model.Service{Name: name, Kind: "sonarr", Version: str("2.0")}
	if err := Upsert(ctx, d, []model.Service{second}, []string{"name"},
		[]string{"kind", "version"}); err != nil {
		t.Fatal(err)
	}

	var got model.Service
	if err := d.DB().WithContext(ctx).First(&got, "name = ?", name).Error; err != nil {
		t.Fatal(err)
	}
	if got.Version == nil || *got.Version != "2.0" {
		t.Errorf("version = %v, want 2.0", got.Version)
	}
	if got.LastSeenAt == nil || !got.LastSeenAt.Equal(seen) {
		t.Errorf("last_seen_at = %v; a column left out of updateCols must survive", got.LastSeenAt)
	}
}

// TestInsertIgnoresConflicts is the append-only path.
func TestInsertIgnoresConflicts(t *testing.T) {
	d, ctx := testDAO(t)
	name := uniqueName("svc")

	rows := []model.Service{{Name: name, Kind: "sonarr", Version: str("1.0")}}
	if err := Insert(ctx, d, rows, []string{"name"}); err != nil {
		t.Fatal(err)
	}
	rows[0].Version = str("2.0")
	if err := Insert(ctx, d, rows, []string{"name"}); err != nil {
		t.Fatalf("a conflicting insert should be a no-op, not an error: %v", err)
	}

	var got model.Service
	if err := d.DB().WithContext(ctx).First(&got, "name = ?", name).Error; err != nil {
		t.Fatal(err)
	}
	if got.Version == nil || *got.Version != "1.0" {
		t.Errorf("version = %v, want the original 1.0", got.Version)
	}
}

// TestUpsertAllReplacesEveryColumn is the snapshot-writer path.
func TestUpsertAllReplacesEveryColumn(t *testing.T) {
	d, ctx := testDAO(t)
	name := uniqueName("svc")
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	if err := UpsertAll(ctx, d, []model.Service{
		{Name: name, Kind: "sonarr", Version: str("1.0"), LastSeenAt: &old},
	}, []string{"name"}); err != nil {
		t.Fatal(err)
	}
	if err := UpsertAll(ctx, d, []model.Service{
		{Name: name, Kind: "sonarr", Version: str("2.0")},
	}, []string{"name"}); err != nil {
		t.Fatal(err)
	}

	var got model.Service
	if err := d.DB().WithContext(ctx).First(&got, "name = ?", name).Error; err != nil {
		t.Fatal(err)
	}
	if got.LastSeenAt != nil {
		t.Errorf("last_seen_at = %v; UpsertAll must replace every column", *got.LastSeenAt)
	}
}

// TestUpsertGuardedOnlyFillsNull is the webhook-versus-snapshot rule.
//
// A webhook may fill a column the snapshot has not learned yet and must never
// overwrite one it already has. Expressed as the statement's own WHERE rather
// than a read-then-write, so two handlers racing cannot interleave.
func TestUpsertGuardedOnlyFillsNull(t *testing.T) {
	d, ctx := testDAO(t)
	hash := uniqueName("hash")

	if err := Insert(ctx, d, []model.Torrent{{Infohash: hash}},
		[]string{"infohash"}); err != nil {
		t.Fatal(err)
	}

	rel := model.Release{Title: uniqueName("rel")}
	if err := d.DB().WithContext(ctx).Create(&rel).Error; err != nil {
		t.Fatal(err)
	}
	other := model.Release{Title: uniqueName("rel")}
	if err := d.DB().WithContext(ctx).Create(&other).Error; err != nil {
		t.Fatal(err)
	}

	// First write fills the NULL.
	if err := UpsertGuarded(ctx, d, []model.Torrent{{Infohash: hash, ReleaseID: &rel.ID}},
		[]string{"infohash"}, "torrents.release_id IS NULL",
		[]string{"release_id"}); err != nil {
		t.Fatal(err)
	}
	// Second write must be refused by the guard.
	if err := UpsertGuarded(ctx, d, []model.Torrent{{Infohash: hash, ReleaseID: &other.ID}},
		[]string{"infohash"}, "torrents.release_id IS NULL",
		[]string{"release_id"}); err != nil {
		t.Fatal(err)
	}

	var got model.Torrent
	if err := d.DB().WithContext(ctx).First(&got, "infohash = ?", hash).Error; err != nil {
		t.Fatal(err)
	}
	if got.ReleaseID == nil || *got.ReleaseID != rel.ID {
		t.Errorf("release_id = %v, want the first writer's %s", got.ReleaseID, rel.ID)
	}
}

// TestUpsertPartialMatchesAPartialIndex is the failure mode this exists for.
//
// A partial unique index only arbitrates when the ON CONFLICT clause repeats its
// predicate verbatim. Naming the columns alone raises "no unique or exclusion
// constraint matching the ON CONFLICT specification".
func TestUpsertPartialMatchesAPartialIndex(t *testing.T) {
	d, ctx := testDAO(t)

	// offers.infohash carries `index:,where:infohash IS NOT NULL`.
	indexer := uniqueName("ix")
	if err := Insert(ctx, d, []model.Indexer{{Name: indexer}},
		[]string{"name"}); err != nil {
		t.Fatal(err)
	}
	hash := uniqueName("h")
	row := model.Offer{
		Kind:         model.OfferAnnounce,
		IndexerName:  indexer,
		SourceRef:    uniqueName("ref"),
		DiscoveredAt: time.Now().UTC(),
		Infohash:     &hash,
	}
	if err := Upsert(ctx, d, []model.Offer{row},
		[]string{"indexer_name", "kind", "source_ref"},
		[]string{"infohash"}); err != nil {
		t.Fatal(err)
	}

	// Repeating the same natural key must update rather than duplicate.
	if err := Upsert(ctx, d, []model.Offer{row},
		[]string{"indexer_name", "kind", "source_ref"},
		[]string{"infohash"}); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := d.DB().WithContext(ctx).Model(&model.Offer{}).
		Where("indexer_name = ? AND source_ref = ?", indexer, row.SourceRef).
		Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("got %d offers, want 1", n)
	}
}

// TestUpsertEmptyIsANoOp keeps every caller from needing a length check.
func TestUpsertEmptyIsANoOp(t *testing.T) {
	d, ctx := testDAO(t)
	if err := Upsert(ctx, d, []model.Service{}, []string{"name"}, []string{"kind"}); err != nil {
		t.Errorf("Upsert: %v", err)
	}
	if err := UpsertAll(ctx, d, []model.Service{}, []string{"name"}); err != nil {
		t.Errorf("UpsertAll: %v", err)
	}
	if err := Insert(ctx, d, []model.Service{}, []string{"name"}); err != nil {
		t.Errorf("Insert: %v", err)
	}
	if err := Create(ctx, d, []model.Service{}); err != nil {
		t.Errorf("Create: %v", err)
	}
	if err := UpsertGuarded(ctx, d, []model.Service{}, []string{"name"}, "true", []string{"kind"}); err != nil {
		t.Errorf("UpsertGuarded: %v", err)
	}
}

// TestFindOneDistinguishesMissFromFailure is the bug it was written for.
//
// The pattern it replaced was `if err != nil { return 0, nil }`, which reported
// a connection failure as a miss — and wrote a permanent null link during every
// database blip.
func TestFindOneDistinguishesMissFromFailure(t *testing.T) {
	d, ctx := testDAO(t)

	var got model.Service
	ok, err := FindOne(ctx, d, &got, "name = ?", uniqueName("absent"))
	if err != nil {
		t.Fatalf("a miss must not be an error: %v", err)
	}
	if ok {
		t.Error("ok = true for a row that does not exist")
	}

	name := uniqueName("svc")
	if err := Insert(ctx, d, []model.Service{{Name: name, Kind: "sonarr"}},
		[]string{"name"}); err != nil {
		t.Fatal(err)
	}
	ok, err = FindOne(ctx, d, &got, "name = ?", name)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if got.Name != name {
		t.Errorf("name = %q", got.Name)
	}

	// A malformed predicate is a failure, not a miss.
	if _, err := FindOne(ctx, d, &got, "no_such_column = ?", 1); err == nil {
		t.Error("a bad query reported a miss instead of an error")
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		t.Error("a bad query was classified as ErrRecordNotFound")
	}
}

// TestTxRollsBackOnError is what keeps a half-written pass out of the database.
func TestTxRollsBackOnError(t *testing.T) {
	d, ctx := testDAO(t)
	name := uniqueName("svc")

	wantErr := errors.New("boom")
	err := d.Tx(ctx, func(tx *DAO) error {
		if err := Insert(ctx, tx, []model.Service{{Name: name, Kind: "sonarr"}},
			[]string{"name"}); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}

	var n int64
	if err := d.DB().WithContext(ctx).Model(&model.Service{}).
		Where("name = ?", name).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("found %d rows; the transaction did not roll back", n)
	}
}

// TestTxCommits is the other half.
func TestTxCommits(t *testing.T) {
	d, ctx := testDAO(t)
	name := uniqueName("svc")

	if err := d.Tx(ctx, func(tx *DAO) error {
		return Insert(ctx, tx, []model.Service{{Name: name, Kind: "sonarr"}},
			[]string{"name"})
	}); err != nil {
		t.Fatal(err)
	}

	var n int64
	if err := d.DB().WithContext(ctx).Model(&model.Service{}).
		Where("name = ?", name).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("found %d rows, want 1", n)
	}
}

// TestBatchingWritesEveryRow checks the CreateInBatches path.
//
// A backfill hands this 127,000 rows at once. Silently writing only the first
// batch would look like a successful pass.
func TestBatchingWritesEveryRow(t *testing.T) {
	d, ctx := testDAO(t)

	const n = batchSize + 17
	prefix := uniqueName("batch")
	rows := make([]model.Service, 0, n)
	for i := range n {
		rows = append(rows, model.Service{
			Name: prefix + "-" + string(rune('a'+i%26)) + "-" + model.NewID().String(),
			Kind: "test",
		})
	}
	if err := Insert(ctx, d, rows, []string{"name"}); err != nil {
		t.Fatal(err)
	}

	var got int64
	if err := d.DB().WithContext(ctx).Model(&model.Service{}).
		Where("name LIKE ?", prefix+"%").Count(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got != n {
		t.Errorf("wrote %d rows, want %d", got, n)
	}
}

// TestRedactionHookFiresOnUpsert is the end-to-end statement of the leak fix.
//
// The hook is on the model, but what matters is that it runs on the path the
// writers actually use — a BeforeSave that never fires on an upsert would be a
// test that passes and a database full of passkeys.
func TestRedactionHookFiresOnUpsert(t *testing.T) {
	d, ctx := testDAO(t)

	indexer := uniqueName("ix")
	if err := Insert(ctx, d, []model.Indexer{{Name: indexer}}, []string{"name"}); err != nil {
		t.Fatal(err)
	}
	ref := uniqueName("ref")
	const secret = "8f2a91c4de77b03e5a16"

	row := model.Offer{
		Kind:         model.OfferAnnounce,
		IndexerName:  indexer,
		SourceRef:    ref,
		DiscoveredAt: time.Now().UTC(),
		DownloadURL:  str("https://tracker.example/dl.php?id=1&passkey=" + secret),
	}
	if err := Upsert(ctx, d, []model.Offer{row},
		[]string{"indexer_name", "kind", "source_ref"},
		[]string{"download_url"}); err != nil {
		t.Fatal(err)
	}

	var got model.Offer
	if err := d.DB().WithContext(ctx).
		First(&got, "indexer_name = ? AND source_ref = ?", indexer, ref).Error; err != nil {
		t.Fatal(err)
	}
	if got.DownloadURL == nil {
		t.Fatal("download_url is nil")
	}
	if strings.Contains(*got.DownloadURL, secret) {
		t.Fatalf("a passkey reached the database: %s", *got.DownloadURL)
	}
}

func str(s string) *string { return &s }

// TestUpsertCoalesceKeepsStoredValuesUnderNull is the per-value rule.
//
// The same release title is announced across several trackers and autobrr's
// parse is not identical every time — a padded name or an incomplete indexer
// definition yields a record with half the fields empty. A plain upsert would
// let the later, worse parse blank what a cleaner one established.
func TestUpsertCoalesceKeepsStoredValuesUnderNull(t *testing.T) {
	d, ctx := testDAO(t)
	title := uniqueName("Some.Release")

	full := model.Release{
		Title:        title,
		Resolution:   str("1080p"),
		Source:       str("WEB-DL"),
		ReleaseGroup: str("GRP"),
	}
	cols := []string{"resolution", "source", "release_group"}
	if err := UpsertCoalesce(ctx, d, []model.Release{full}, []string{"title"}, cols); err != nil {
		t.Fatal(err)
	}

	// A second announce of the same title that parsed badly: source survived,
	// the other two did not.
	partial := model.Release{Title: title, Source: str("BluRay")}
	if err := UpsertCoalesce(ctx, d, []model.Release{partial}, []string{"title"}, cols); err != nil {
		t.Fatal(err)
	}

	var got model.Release
	if err := d.DB().WithContext(ctx).First(&got, "title = ?", title).Error; err != nil {
		t.Fatal(err)
	}
	if got.Resolution == nil || *got.Resolution != "1080p" {
		t.Errorf("resolution = %v; a NULL must not overwrite a stored value", got.Resolution)
	}
	if got.ReleaseGroup == nil || *got.ReleaseGroup != "GRP" {
		t.Errorf("release_group = %v", got.ReleaseGroup)
	}
	// A value that IS present still wins: this is COALESCE, not first-write.
	if got.Source == nil || *got.Source != "BluRay" {
		t.Errorf("source = %v, want the newer BluRay", got.Source)
	}
}

// TestUpsertCoalesceResolvesTheTableName guards the rendered SQL.
//
// The assignment is `col = COALESCE(EXCLUDED.col, table.col)` and the table name
// is taken from the model. A wrong name is a syntax error at execution time, not
// at compile time, so it is worth one round trip to prove.
func TestUpsertCoalesceResolvesTheTableName(t *testing.T) {
	d, ctx := testDAO(t)
	if err := UpsertCoalesce(ctx, d,
		[]model.Service{{Name: uniqueName("svc"), Kind: "test", Version: str("1")}},
		[]string{"name"}, []string{"kind", "version"}); err != nil {
		t.Fatalf("UpsertCoalesce against services: %v", err)
	}
}

func TestUpsertCoalesceEmptyIsANoOp(t *testing.T) {
	d, ctx := testDAO(t)
	if err := UpsertCoalesce(ctx, d, []model.Release{}, []string{"title"},
		[]string{"resolution"}); err != nil {
		t.Errorf("UpsertCoalesce: %v", err)
	}
}

// TestUpsertCounterAccumulates is the difference between a state and a count.
//
// Every other writer here records the latest value of something, and replacing
// is right for those. A counter is the opposite: each write is one more
// occurrence, and assigning would make an album that failed to import five
// times indistinguishable from one that failed once — the only question such a
// table exists to answer.
func TestUpsertCounterAccumulates(t *testing.T) {
	d, ctx := testDAO(t)

	content := model.Content{Kind: model.KindAlbum, Title: uniqueName("Album")}
	if err := d.DB().WithContext(ctx).Create(&content).Error; err != nil {
		t.Fatal(err)
	}

	one := 1
	first := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)

	for _, at := range []time.Time{first, later, later} {
		if err := UpsertCounter(ctx, d, []model.SoularrEvent{{
			ContentID: content.ID, Event: "import_failed",
			Count: &one, LastSeenAt: at,
		}}, []string{"content_id", "event"}, "count",
			[]string{"last_seen_at"}); err != nil {
			t.Fatal(err)
		}
	}

	var got model.SoularrEvent
	if err := d.DB().WithContext(ctx).
		First(&got, "content_id = ? AND event = ?", content.ID, "import_failed").
		Error; err != nil {
		t.Fatal(err)
	}
	if got.Count == nil || *got.Count != 3 {
		t.Errorf("count = %v, want 3", got.Count)
	}
	// The non-counter columns still take the newest value.
	if !got.LastSeenAt.Equal(later) {
		t.Errorf("last_seen_at = %v, want %v", got.LastSeenAt, later)
	}
}

// TestUpsertCounterStartsFromZeroNotNull covers a row whose counter is unset.
func TestUpsertCounterStartsFromZeroNotNull(t *testing.T) {
	d, ctx := testDAO(t)

	content := model.Content{Kind: model.KindAlbum, Title: uniqueName("Album")}
	if err := d.DB().WithContext(ctx).Create(&content).Error; err != nil {
		t.Fatal(err)
	}
	// Seed a row with a NULL count, the way an older writer would have.
	if err := d.DB().WithContext(ctx).Create(&model.SoularrEvent{
		ContentID: content.ID, Event: "import_failed",
		LastSeenAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatal(err)
	}

	one := 1
	if err := UpsertCounter(ctx, d, []model.SoularrEvent{{
		ContentID: content.ID, Event: "import_failed",
		Count: &one, LastSeenAt: time.Now().UTC(),
	}}, []string{"content_id", "event"}, "count",
		[]string{"last_seen_at"}); err != nil {
		t.Fatal(err)
	}

	var got model.SoularrEvent
	if err := d.DB().WithContext(ctx).
		First(&got, "content_id = ? AND event = ?", content.ID, "import_failed").
		Error; err != nil {
		t.Fatal(err)
	}
	if got.Count == nil || *got.Count != 1 {
		t.Errorf("count = %v; a NULL start must count from zero", got.Count)
	}
}

// TestUpsertDeduplicatesWithinABatch is the failure that took a backfill down.
//
// Postgres refuses a single INSERT ... ON CONFLICT DO UPDATE that would affect
// one row twice — 21000, "command cannot affect row a second time". A page of
// autobrr releases holds one title twice whenever the release was announced on
// two trackers, which is exactly what cross-seeding produces, so this is the
// normal case rather than an edge one.
func TestUpsertDeduplicatesWithinABatch(t *testing.T) {
	d, ctx := testDAO(t)
	name := uniqueName("svc")

	// The same conflict key three times in one call.
	rows := []model.Service{
		{Name: name, Kind: "sonarr", Version: str("1.0")},
		{Name: name, Kind: "sonarr", Version: str("2.0")},
		{Name: name, Kind: "sonarr", Version: str("3.0")},
	}
	if err := Upsert(ctx, d, rows, []string{"name"},
		[]string{"kind", "version"}); err != nil {
		t.Fatalf("a batch with a repeated key must not fail: %v", err)
	}

	var got model.Service
	if err := d.DB().WithContext(ctx).First(&got, "name = ?", name).Error; err != nil {
		t.Fatal(err)
	}
	// Last wins: the later row is the more recent observation.
	if got.Version == nil || *got.Version != "3.0" {
		t.Errorf("version = %v, want the last of the batch", got.Version)
	}
}

// TestUpsertDeduplicatesOnCompositeKeys covers a multi-column arbiter.
func TestUpsertDeduplicatesOnCompositeKeys(t *testing.T) {
	d, ctx := testDAO(t)
	svc := uniqueName("svc")
	if err := Insert(ctx, d, []model.Service{{Name: svc, Kind: "sonarr"}},
		[]string{"name"}); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	rows := []model.HealthEvent{
		{ServiceName: svc, Type: "IndexerStatusCheck", Source: "warning",
			Message: str("first"), FirstSeenAt: now, LastSeenAt: now},
		{ServiceName: svc, Type: "IndexerStatusCheck", Source: "warning",
			Message: str("second"), FirstSeenAt: now, LastSeenAt: now},
		// Same service and type, DIFFERENT source: a distinct row that must
		// survive the deduplication.
		{ServiceName: svc, Type: "IndexerStatusCheck", Source: "error",
			Message: str("other"), FirstSeenAt: now, LastSeenAt: now},
	}
	if err := Upsert(ctx, d, rows,
		[]string{"service_name", "type", "source"},
		[]string{"message", "last_seen_at"}); err != nil {
		t.Fatalf("composite-key batch failed: %v", err)
	}

	var n int64
	if err := d.DB().WithContext(ctx).Model(&model.HealthEvent{}).
		Where("service_name = ?", svc).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("got %d rows, want 2 — the differing source was dropped", n)
	}

	var got model.HealthEvent
	if err := d.DB().WithContext(ctx).
		First(&got, "service_name = ? AND source = ?", svc, "warning").Error; err != nil {
		t.Fatal(err)
	}
	if got.Message == nil || *got.Message != "second" {
		t.Errorf("message = %v, want the last of the duplicates", got.Message)
	}
}

// TestUpsertCoalesceDeduplicates proves the guard covers the other writers too,
// not just plain Upsert.
func TestUpsertCoalesceDeduplicates(t *testing.T) {
	d, ctx := testDAO(t)
	title := uniqueName("Some.Release")

	rows := []model.Release{
		{Title: title, Resolution: str("1080p")},
		{Title: title, Source: str("BluRay")},
	}
	if err := UpsertCoalesce(ctx, d, rows, []string{"title"},
		[]string{"resolution", "source"}); err != nil {
		t.Fatalf("UpsertCoalesce with a repeated key failed: %v", err)
	}

	var got model.Release
	if err := d.DB().WithContext(ctx).First(&got, "title = ?", title).Error; err != nil {
		t.Fatal(err)
	}
	// Last wins inside the batch, then COALESCE applies against what is stored.
	// Merging partial rows is the caller's job — releaseRows does it — and this
	// only has to not blow up.
	if got.Source == nil || *got.Source != "BluRay" {
		t.Errorf("source = %v", got.Source)
	}
}

// TestUpsertCounterDeduplicates checks the counter path does not double-count.
func TestUpsertCounterDeduplicates(t *testing.T) {
	d, ctx := testDAO(t)

	content := model.Content{Kind: model.KindAlbum, Title: uniqueName("Album")}
	if err := d.DB().WithContext(ctx).Create(&content).Error; err != nil {
		t.Fatal(err)
	}

	one := 1
	now := time.Now().UTC()
	rows := []model.SoularrEvent{
		{ContentID: content.ID, Event: "import_failed", Count: &one, LastSeenAt: now},
		{ContentID: content.ID, Event: "import_failed", Count: &one, LastSeenAt: now},
	}
	if err := UpsertCounter(ctx, d, rows, []string{"content_id", "event"},
		"count", []string{"last_seen_at"}); err != nil {
		t.Fatalf("UpsertCounter with a repeated key failed: %v", err)
	}

	var got model.SoularrEvent
	if err := d.DB().WithContext(ctx).
		First(&got, "content_id = ?", content.ID).Error; err != nil {
		t.Fatal(err)
	}
	// One survives the deduplication, so the counter advances by one. A caller
	// that means "two occurrences" sends a count of two.
	if got.Count == nil || *got.Count != 1 {
		t.Errorf("count = %v, want 1", got.Count)
	}
}

// TestDedupeLeavesDistinctRowsAlone is the property that matters most: the
// guard must never drop a row that was genuinely different.
func TestDedupeLeavesDistinctRowsAlone(t *testing.T) {
	d, ctx := testDAO(t)

	const n = 50
	prefix := uniqueName("svc")
	rows := make([]model.Service, 0, n)
	for i := range n {
		rows = append(rows, model.Service{
			Name: fmt.Sprintf("%s-%d", prefix, i), Kind: "test",
		})
	}
	if err := Upsert(ctx, d, rows, []string{"name"}, []string{"kind"}); err != nil {
		t.Fatal(err)
	}

	var got int64
	if err := d.DB().WithContext(ctx).Model(&model.Service{}).
		Where("name LIKE ?", prefix+"%").Count(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got != n {
		t.Errorf("wrote %d rows, want %d", got, n)
	}
}

// The production failure InsertIgnore exists for.
//
// content_external_ids carries TWO unique constraints — a primary key on
// (content_id, provider) and a unique index on (provider, value). Insert names
// ONE arbiter; Postgres enforces the other normally, so a row colliding on the
// unnamed index raises 23505 that no ON CONFLICT catches.
//
// This killed the sonarr reconcile for nineteen hours, and because a failing
// pass aborts the ones after it, radarr and lidarr never ran either.
func TestInsertWithWrongArbiterStillRaises(t *testing.T) {
	d, ctx := testDAO(t)
	contentID := model.NewID()
	provider := uniqueName("prov")

	seed := []model.ContentExternalID{
		{ContentID: contentID, Provider: provider, Value: uniqueName("v1")},
	}
	if err := Insert(ctx, d, seed, []string{"provider", "value"}); err != nil {
		t.Fatal(err)
	}

	// Same content+provider, DIFFERENT value: passes the (provider, value)
	// arbiter untouched and lands on the primary key instead.
	clash := []model.ContentExternalID{
		{ContentID: contentID, Provider: provider, Value: uniqueName("v2")},
	}
	err := Insert(ctx, d, clash, []string{"provider", "value"})
	if err == nil {
		t.Fatal("expected the primary key to reject this; if it no longer does, " +
			"the schema changed and InsertIgnore's rationale needs revisiting")
	}

	// InsertIgnore names no arbiter, so ANY unique violation is skipped.
	if err := InsertIgnore(ctx, d, clash); err != nil {
		t.Errorf("InsertIgnore should absorb the conflict, got %v", err)
	}
}

// Duplicates WITHIN one batch are tolerated, which DO UPDATE cannot manage:
// Postgres refuses to affect a row twice in one statement, but only when there
// is an update to apply. DO NOTHING keeps the first and skips the rest.
func TestInsertIgnoreToleratesDuplicatesInOneBatch(t *testing.T) {
	d, ctx := testDAO(t)
	contentID := model.NewID()
	provider := uniqueName("prov")

	batch := []model.ContentExternalID{
		{ContentID: contentID, Provider: provider, Value: uniqueName("a")},
		{ContentID: contentID, Provider: provider, Value: uniqueName("b")},
	}
	if err := InsertIgnore(ctx, d, batch); err != nil {
		t.Fatalf("a batch carrying two values for one content+provider must not "+
			"fail the pass: %v", err)
	}

	var n int64
	err := d.DB().WithContext(ctx).Model(&model.ContentExternalID{}).
		Where("content_id = ? AND provider = ?", contentID, provider).
		Count(&n).Error
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("rows = %d, want the first kept and the rest skipped", n)
	}
}

// An empty batch is a no-op rather than a malformed statement.
func TestInsertIgnoreEmpty(t *testing.T) {
	d, ctx := testDAO(t)
	if err := InsertIgnore(ctx, d, []model.ContentExternalID{}); err != nil {
		t.Errorf("empty batch: %v", err)
	}
}
