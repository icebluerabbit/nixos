package cleanup

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"homelab/clients/qbittorrent"
	"homelab/httpx"
	"homelab/managarr/internal/model"
	"homelab/managarr/internal/store"
)

// Against a real PostgreSQL, like the rest of managarr's data tests: the
// candidate query names tables and columns by hand, and a renamed column is
// invisible to anything but the database.
//
//	MANAGARR_TEST_DSN=postgres://managarr@host:5432/managarr_test?sslmode=disable go test ./...
const testDSNEnv = "MANAGARR_TEST_DSN"

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testDB(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Skipf("%s not set", testDSNEnv)
	}
	ctx := context.Background()
	db, err := store.OpenGorm(ctx, dsn, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AutoMigrate(db, discardLogger()); err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

// fakeClient records what was asked of qBittorrent without a real one.
type fakeClient struct {
	srv      *httptest.Server
	deleted  []string
	withData bool
	calls    int
}

func newFakeClient(t *testing.T) (*qbittorrent.Client, *fakeClient) {
	t.Helper()
	f := &fakeClient{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "torrents/delete") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		f.calls++
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		f.deleted = append(f.deleted, strings.Split(form.Get("hashes"), "|")...)
		f.withData = form.Get("deleteFiles") == "true"
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(f.srv.Close)

	c, err := qbittorrent.New(httpx.Config{BaseURL: f.srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return c, f
}

// seed writes one client torrent and one tracker row for it.
func seed(t *testing.T, db *gorm.DB, ctx context.Context, client, hash, host, message string, code int) {
	t.Helper()
	err := db.WithContext(ctx).Exec(`
		INSERT INTO clients (name, kind) VALUES (?, 'qbittorrent')
		ON CONFLICT (name) DO NOTHING`, client).Error
	if err != nil {
		t.Fatal(err)
	}
	// The parent row. client_torrents and torrent_trackers both reference
	// torrents, so the torrent has to exist before either — the same order
	// the reconcile writes them in.
	err = db.WithContext(ctx).Exec(`
		INSERT INTO torrents (infohash) VALUES (?)
		ON CONFLICT (infohash) DO NOTHING`, hash).Error
	if err != nil {
		t.Fatal(err)
	}
	err = db.WithContext(ctx).Exec(`
		INSERT INTO client_torrents (client_name, torrent_infohash, content_path, category, downloaded_bytes)
		VALUES (?, ?, ?, 'tv', 1000)
		ON CONFLICT (client_name, torrent_infohash) DO NOTHING`,
		client, hash, "/mnt/media/complete/"+hash).Error
	if err != nil {
		t.Fatal(err)
	}
	err = db.WithContext(ctx).Exec(`
		INSERT INTO trackers (announce_host) VALUES (?)
		ON CONFLICT (announce_host) DO NOTHING`, host).Error
	if err != nil {
		t.Fatal(err)
	}
	err = db.WithContext(ctx).Exec(`
		INSERT INTO torrent_trackers (torrent_infohash, tracker_announce_host, status, status_code, message, url)
		VALUES (?, ?, 'error', ?, ?, ?)
		ON CONFLICT (torrent_infohash, tracker_announce_host) DO UPDATE
		   SET status_code = EXCLUDED.status_code, message = EXCLUDED.message`,
		hash, host, code, message, "https://"+host+"/announce").Error
	if err != nil {
		t.Fatal(err)
	}
}

// testClient names a client no other test and no earlier run will use.
//
// Both halves are load-bearing. The test name separates tests within a run;
// without it they share a client and see each other's rows. The random suffix
// separates RUNS; this database is shared and never truncated, so a name that
// is merely unique per test still collides with everything the last run left
// behind.
//
// The suffix comes from the END of the id. UUIDv7 encodes its timestamp in the
// LEADING bits, so ids minted in the same millisecond share their first
// characters — slicing the front produces the collision it was meant to avoid.
func testClient(t *testing.T) string {
	t.Helper()
	id := strings.ReplaceAll(model.NewID().String(), "-", "")
	return "qbt-" + t.Name() + "-" + id[len(id)-8:]
}

// uniqueHash builds a 40-character infohash that no other run will produce.
//
// These tests write into a shared database and never truncate it, so a fixed
// hash would collide with every previous run's rows and make the candidate
// counts depend on test history.
func uniqueHash(t *testing.T) string {
	t.Helper()
	// A UUID without its dashes is 32 hex characters; an infohash is 40.
	return strings.ReplaceAll(model.NewID().String(), "-", "") + "00000000"
}

// The rule, end to end: a tracker saying the torrent is gone gets it deleted
// WITH its data.
func TestRunDeletesUnregisteredWithData(t *testing.T) {
	db, ctx := testDB(t)
	client := testClient(t)
	hash := uniqueHash(t)
	seed(t, db, ctx, client, hash, "tracker.example", "unregistered torrent", qbittorrent.TrackerError)

	qb, fake := newFakeClient(t)
	st, err := New(db, qb, discardLogger(), client).Run(ctx, false)
	if err != nil {
		t.Fatal(err)
	}

	if st.Candidates != 1 || st.Deleted != 1 {
		t.Fatalf("stats = %+v, want one candidate deleted", st)
	}
	if len(fake.deleted) != 1 || fake.deleted[0] != hash {
		t.Errorf("deleted %v, want %s", fake.deleted, hash)
	}
	// With data. An *arr import or a cross-seed inject is a hardlink, so this
	// unlinks qBittorrent's copy and leaves theirs — the bytes only go when
	// nothing else holds a link.
	if !fake.withData {
		t.Error("deleted without data; the whole point is reclaiming the space")
	}
}

// The deletion has to leave evidence. Without it, all a later pass can see is
// that a torrent is absent — which is the same observation whether a tracker
// dropped it, an *arr cleaned it up, or somebody removed it by hand.
//
// That distinction is what makes deleting an orphaned library entry defensible
// rather than reckless, so it is recorded at the one moment it is known.
func TestRunRecordsWhatItDeleted(t *testing.T) {
	db, ctx := testDB(t)
	client := testClient(t)
	hash := uniqueHash(t)
	seed(t, db, ctx, client, hash, "tracker.example", "unregistered torrent", qbittorrent.TrackerError)

	qb, _ := newFakeClient(t)
	if _, err := New(db, qb, discardLogger(), client).Run(ctx, false); err != nil {
		t.Fatal(err)
	}

	var rec model.TorrentRemoval
	err := db.WithContext(ctx).
		Where("infohash = ? AND client_name = ?", hash, client).
		Take(&rec).Error
	if err != nil {
		t.Fatalf("no removal recorded: %v", err)
	}
	if rec.Reason != ReasonUnregistered {
		t.Errorf("reason = %q, want %q", rec.Reason, ReasonUnregistered)
	}
	// The tracker's own wording, kept verbatim. It is the evidence, and it is
	// what makes a wrong deletion diagnosable rather than merely regrettable.
	if rec.Message != "unregistered torrent" {
		t.Errorf("message = %q, want the tracker's wording", rec.Message)
	}
	if rec.Tracker != "tracker.example" {
		t.Errorf("tracker = %q", rec.Tracker)
	}
	if !rec.DeletedData {
		t.Error("recorded as data-preserving; this cleaner always deletes data")
	}
	if rec.Path == nil || *rec.Path != "/mnt/media/complete/"+hash {
		t.Errorf("path = %v, want the content path", rec.Path)
	}
	if rec.SizeBytes == nil || *rec.SizeBytes != 1000 {
		t.Errorf("size = %v, want what the client downloaded", rec.SizeBytes)
	}
	if rec.RemovedAt.IsZero() {
		t.Error("removed_at unset")
	}
}

// The reason also lands on client_torrents, where a join deciding whether a
// library file is orphaned reaches it without a second lookup.
//
// removed_at stays NULL: the reconcile owns that column, and having two writers
// disagree about what the client holds is worse than the extra hour's lag.
func TestRunMarksTheRemovalReason(t *testing.T) {
	db, ctx := testDB(t)
	client := testClient(t)
	hash := uniqueHash(t)
	seed(t, db, ctx, client, hash, "tracker.example", "torrent not registered", qbittorrent.TrackerError)

	qb, _ := newFakeClient(t)
	if _, err := New(db, qb, discardLogger(), client).Run(ctx, false); err != nil {
		t.Fatal(err)
	}

	var got struct {
		RemovalReason *string
		RemovedAt     *time.Time
	}
	err := db.WithContext(ctx).Raw(`
		SELECT removal_reason, removed_at FROM client_torrents
		 WHERE client_name = ? AND torrent_infohash = ?`, client, hash).Scan(&got).Error
	if err != nil {
		t.Fatal(err)
	}
	if got.RemovalReason == nil || *got.RemovalReason != ReasonUnregistered {
		t.Errorf("removal_reason = %v, want %q", got.RemovalReason, ReasonUnregistered)
	}
	if got.RemovedAt != nil {
		t.Error("cleanup set removed_at; that column belongs to the reconcile")
	}
}

// A dry run records nothing, for the same reason it deletes nothing: a record
// of a deletion that did not happen is worse than no record, because everything
// downstream treats it as fact.
func TestDryRunRecordsNothing(t *testing.T) {
	db, ctx := testDB(t)
	client := testClient(t)
	hash := uniqueHash(t)
	seed(t, db, ctx, client, hash, "tracker.example", "unregistered torrent", qbittorrent.TrackerError)

	qb, _ := newFakeClient(t)
	if _, err := New(db, qb, discardLogger(), client).Run(ctx, true); err != nil {
		t.Fatal(err)
	}

	var n int64
	err := db.WithContext(ctx).Model(&model.TorrentRemoval{}).
		Where("client_name = ?", client).Count(&n).Error
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("a dry run wrote %d removal records", n)
	}

	var reason *string
	err = db.WithContext(ctx).Raw(`
		SELECT removal_reason FROM client_torrents
		 WHERE client_name = ? AND torrent_infohash = ?`, client, hash).Scan(&reason).Error
	if err != nil {
		t.Fatal(err)
	}
	if reason != nil {
		t.Errorf("a dry run set removal_reason = %q", *reason)
	}
}

// A torrent announcing to several trackers is deleted once, so it gets ONE
// record. Two rows would double every count taken from this table.
func TestMultiTrackerTorrentIsRecordedOnce(t *testing.T) {
	db, ctx := testDB(t)
	client := testClient(t)
	hash := uniqueHash(t)
	seed(t, db, ctx, client, hash, "a.example", "unregistered torrent", qbittorrent.TrackerError)
	seed(t, db, ctx, client, hash, "b.example", "torrent not registered", qbittorrent.TrackerError)

	qb, _ := newFakeClient(t)
	if _, err := New(db, qb, discardLogger(), client).Run(ctx, false); err != nil {
		t.Fatal(err)
	}

	var n int64
	err := db.WithContext(ctx).Model(&model.TorrentRemoval{}).
		Where("infohash = ? AND client_name = ?", hash, client).Count(&n).Error
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("recorded %d removals for one deletion", n)
	}
}

// A refused delete must leave no record. The torrents are still in the client,
// and a record saying otherwise is a lie that outlives the failure.
func TestFailedDeleteRecordsNothing(t *testing.T) {
	db, ctx := testDB(t)
	client := testClient(t)
	hash := uniqueHash(t)
	seed(t, db, ctx, client, hash, "tracker.example", "unregistered torrent", qbittorrent.TrackerError)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	qb, err := qbittorrent.New(httpx.Config{BaseURL: srv.URL, RetryMax: -1})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := New(db, qb, discardLogger(), client).Run(ctx, false); err == nil {
		t.Fatal("expected an error when the client refuses")
	}

	var n int64
	err = db.WithContext(ctx).Model(&model.TorrentRemoval{}).
		Where("client_name = ?", client).Count(&n).Error
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("recorded %d removals for a delete that failed", n)
	}
}

// A dry run must find the same torrents and touch nothing. It is the default
// at every layer above this, because the action is irreversible and the
// evidence is a string a tracker sent.
func TestDryRunDeletesNothing(t *testing.T) {
	db, ctx := testDB(t)
	client := testClient(t)
	hash := uniqueHash(t)
	seed(t, db, ctx, client, hash, "tracker.example", "Torrent has been deleted.", qbittorrent.TrackerError)

	qb, fake := newFakeClient(t)
	st, err := New(db, qb, discardLogger(), client).Run(ctx, true)
	if err != nil {
		t.Fatal(err)
	}

	if st.Candidates != 1 {
		t.Errorf("candidates = %d, want 1", st.Candidates)
	}
	if st.Deleted != 0 {
		t.Errorf("deleted = %d on a dry run", st.Deleted)
	}
	if fake.calls != 0 {
		t.Error("a dry run called qBittorrent")
	}
	if !st.DryRun {
		t.Error("stats do not record that this was a dry run")
	}
}

// The failure that would cost real data: code 5 is a GENERIC tracker error and
// includes messages that are not deletions at all. Acting on the code alone
// would delete a torrent whose tracker is working perfectly.
func TestPeerLimitNoticeIsNotADeletion(t *testing.T) {
	db, ctx := testDB(t)
	client := testClient(t)
	hash := uniqueHash(t)
	seed(t, db, ctx, client, hash, "tracker.example",
		"You already have 3 peers on this torrent. Ignoring.", qbittorrent.TrackerError)

	qb, fake := newFakeClient(t)
	st, err := New(db, qb, discardLogger(), client).Run(ctx, false)
	if err != nil {
		t.Fatal(err)
	}

	if st.Candidates != 0 {
		t.Errorf("candidates = %d; a peer-limit notice is not a deletion", st.Candidates)
	}
	if st.Unmatched != 1 {
		t.Errorf("unmatched = %d, want the message surfaced rather than dropped", st.Unmatched)
	}
	if len(fake.deleted) != 0 {
		t.Fatalf("DELETED A HEALTHY TORRENT: %v", fake.deleted)
	}
}

// A working tracker is not a candidate whatever its message says.
func TestWorkingTrackerIsIgnored(t *testing.T) {
	db, ctx := testDB(t)
	client := testClient(t)
	hash := uniqueHash(t)
	seed(t, db, ctx, client, hash, "tracker.example", "unregistered torrent", qbittorrent.TrackerWorking)

	qb, fake := newFakeClient(t)
	st, err := New(db, qb, discardLogger(), client).Run(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.Candidates != 0 || len(fake.deleted) != 0 {
		t.Errorf("acted on a working tracker: %+v %v", st, fake.deleted)
	}
}

// One torrent announcing to several trackers has a row per tracker. It must be
// deleted once — a repeated hash in the request is at best noise and at worst
// an error from the client.
func TestMultiTrackerTorrentIsDeletedOnce(t *testing.T) {
	db, ctx := testDB(t)
	client := testClient(t)
	hash := uniqueHash(t)
	seed(t, db, ctx, client, hash, "a.example", "unregistered torrent", qbittorrent.TrackerError)
	seed(t, db, ctx, client, hash, "b.example", "torrent not registered", qbittorrent.TrackerError)

	qb, fake := newFakeClient(t)
	st, err := New(db, qb, discardLogger(), client).Run(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.Candidates != 1 {
		t.Errorf("candidates = %d, want the torrent counted once", st.Candidates)
	}
	if len(fake.deleted) != 1 {
		t.Errorf("deleted %v, want one hash", fake.deleted)
	}
}

// Another client's torrents are not this cleaner's to delete. Scoping is by
// client_name so a second torrent client cannot be emptied through the wrong
// API.
func TestOtherClientsAreNotTouched(t *testing.T) {
	db, ctx := testDB(t)
	mine := testClient(t)
	theirs := testClient(t) + "-other"
	hash := uniqueHash(t)
	seed(t, db, ctx, theirs, hash, "tracker.example", "unregistered torrent", qbittorrent.TrackerError)

	qb, fake := newFakeClient(t)
	st, err := New(db, qb, discardLogger(), mine).Run(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.Candidates != 0 || len(fake.deleted) != 0 {
		t.Errorf("deleted another client's torrent: %+v %v", st, fake.deleted)
	}
}

// A torrent already gone from the client is not a candidate: removed_at set
// means a pass has already observed it leave.
func TestRemovedTorrentsAreIgnored(t *testing.T) {
	db, ctx := testDB(t)
	client := testClient(t)
	hash := uniqueHash(t)
	seed(t, db, ctx, client, hash, "tracker.example", "unregistered torrent", qbittorrent.TrackerError)

	err := db.WithContext(ctx).Exec(`
		UPDATE client_torrents SET removed_at = now()
		 WHERE client_name = ? AND torrent_infohash = ?`, client, hash).Error
	if err != nil {
		t.Fatal(err)
	}

	qb, fake := newFakeClient(t)
	st, err := New(db, qb, discardLogger(), client).Run(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.Candidates != 0 || len(fake.deleted) != 0 {
		t.Errorf("acted on an already-removed torrent: %+v %v", st, fake.deleted)
	}
}

// Nothing to do is not an error, and must not call the client at all.
func TestNoCandidatesMakesNoCall(t *testing.T) {
	db, ctx := testDB(t)
	qb, fake := newFakeClient(t)

	st, err := New(db, qb, discardLogger(), testClient(t)).Run(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.Candidates != 0 || st.Deleted != 0 || fake.calls != 0 {
		t.Errorf("stats = %+v, calls = %d", st, fake.calls)
	}
}

// A client that refuses the delete must surface it rather than reporting
// success: the rows stay, and the next run tries again.
func TestClientFailureIsReported(t *testing.T) {
	db, ctx := testDB(t)
	client := testClient(t)
	hash := uniqueHash(t)
	seed(t, db, ctx, client, hash, "tracker.example", "unregistered torrent", qbittorrent.TrackerError)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	qb, err := qbittorrent.New(httpx.Config{BaseURL: srv.URL, RetryMax: -1})
	if err != nil {
		t.Fatal(err)
	}

	st, err := New(db, qb, discardLogger(), client).Run(ctx, false)
	if err == nil {
		t.Fatal("expected an error when the client refuses")
	}
	if st.Deleted != 0 {
		t.Errorf("reported %d deleted after a failure", st.Deleted)
	}
}
