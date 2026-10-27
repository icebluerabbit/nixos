package orphan

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"gorm.io/gorm"

	"homelab/managarr/internal/model"
	"homelab/managarr/internal/store"
)

// Against a real PostgreSQL, like the rest of managarr's data tests. The
// question this package answers is a graph walk over three tables, and an
// in-memory stand-in would be testing the stand-in.
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

// fakeArr records what was asked of an *arr, and can refuse.
type fakeArr struct {
	deleted []int64
	err     error
}

func (f *fakeArr) DeleteFile(_ context.Context, id int64) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)
	return nil
}

// uniqueHash builds a 40-character infohash no other run will produce.
//
// These tests write into a shared database that is never truncated, so a fixed
// hash would collide with every previous run and make the counts depend on test
// history.
func uniqueHash(t *testing.T) string {
	t.Helper()
	// A UUID without its dashes is 32 hex characters; an infohash is 40.
	return strings.ReplaceAll(model.NewID().String(), "-", "") + "00000000"
}

// uniquePath names a library file no other run will have used, for the same
// reason as uniqueHash — path is the unique key on library_files.
func uniquePath(t *testing.T) string {
	t.Helper()
	return "/mnt/media/library/tv/" + t.Name() + "/" + model.NewID().String() + ".mkv"
}

// clientName is unique per test AND per run.
//
// The suffix comes from the END of the id. UUIDv7 encodes its timestamp in the
// LEADING bits, so ids minted in the same millisecond share their first
// characters and slicing the front produces the collision it was meant to
// avoid.
func clientName(t *testing.T) string {
	t.Helper()
	id := strings.ReplaceAll(model.NewID().String(), "-", "")
	return "qbt-" + t.Name() + "-" + id[len(id)-8:]
}

// world builds one scenario: a torrent, optionally held by a client, optionally
// removed by managarr, and a library file imported from it.
type world struct {
	db     *gorm.DB
	ctx    context.Context
	t      *testing.T
	client string
}

func newWorld(t *testing.T) *world {
	db, ctx := testDB(t)
	w := &world{db: db, ctx: ctx, t: t, client: clientName(t)}
	err := db.WithContext(ctx).Exec(`
		INSERT INTO clients (name, kind) VALUES (?, 'qbittorrent')
		ON CONFLICT (name) DO NOTHING`, w.client).Error
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// torrent records a torrent, and whether a client still holds it.
func (w *world) torrent(hash string, held bool) {
	w.t.Helper()
	err := w.db.WithContext(w.ctx).Exec(`
		INSERT INTO torrents (infohash) VALUES (?)
		ON CONFLICT (infohash) DO NOTHING`, hash).Error
	if err != nil {
		w.t.Fatal(err)
	}
	if !held {
		return
	}
	err = w.db.WithContext(w.ctx).Exec(`
		INSERT INTO client_torrents (client_name, torrent_infohash)
		VALUES (?, ?)
		ON CONFLICT (client_name, torrent_infohash) DO NOTHING`,
		w.client, hash).Error
	if err != nil {
		w.t.Fatal(err)
	}
}

// removed records that managarr deleted the torrent, which is what makes its
// library file a candidate at all.
func (w *world) removed(hash string) {
	w.t.Helper()
	err := w.db.WithContext(w.ctx).Exec(`
		INSERT INTO torrent_removals
		    (id, infohash, client_name, reason, tracker, message, deleted_data, removed_at)
		VALUES (?, ?, ?, 'unregistered', 'tracker.example', 'unregistered torrent', true, now())`,
		model.NewID(), hash, w.client).Error
	if err != nil {
		w.t.Fatal(err)
	}
}

// file records a library file imported from hash, and returns its path.
func (w *world) file(service, hash string, arrFileID int) string {
	w.t.Helper()
	path := uniquePath(w.t)
	err := w.db.WithContext(w.ctx).Exec(`
		INSERT INTO library_files (id, path, service_name, torrent_infohash, arr_file_id)
		VALUES (?, ?, ?, ?, ?)`,
		model.NewID(), path, service, hash, arrFileID).Error
	if err != nil {
		w.t.Fatal(err)
	}
	return path
}

// crossSeed records cross-seed's own statement that two infohashes carry the
// same content — a searchee and a candidate it found for it.
func (w *world) crossSeed(source, candidate string) {
	w.t.Helper()
	err := w.db.WithContext(w.ctx).Exec(`
		INSERT INTO offers (id, kind, indexer_name, source_ref, source_infohash, infohash, discovered_at)
		VALUES (?, 'cross_seed_candidate', NULL, ?, ?, ?, now())`,
		model.NewID(), model.NewID().String(), source, candidate).Error
	if err != nil {
		w.t.Fatal(err)
	}
}

func (w *world) remover(clients map[string]Deleter) *Remover {
	return New(w.db, discardLogger(), clients)
}

// The base case: the only torrent that provided this file is gone, so the file
// is orphaned and the *arr is told to delete it.
func TestDeletesFileWithNoProviderLeft(t *testing.T) {
	w := newWorld(t)
	hash := uniqueHash(t)
	w.torrent(hash, false)
	w.removed(hash)
	path := w.file("sonarr", hash, 4242)

	sonarr := &fakeArr{}
	st, err := w.remover(map[string]Deleter{"sonarr": sonarr}).Run(w.ctx, false)
	if err != nil {
		t.Fatal(err)
	}

	if !containsPath(st.Files, path) {
		t.Fatalf("orphan not found; files = %+v", st.Files)
	}
	if len(sonarr.deleted) != 1 || sonarr.deleted[0] != 4242 {
		t.Errorf("deleted %v, want [4242]", sonarr.deleted)
	}
	if st.Deleted != 1 {
		t.Errorf("deleted = %d, want 1", st.Deleted)
	}
}

// The case that makes this safe rather than reckless: the named torrent is
// gone, but cross-seed injected the same content from another tracker and that
// one is still seeding. The file has a provider, so it stays.
func TestKeepsFileWithLiveCrossSeed(t *testing.T) {
	w := newWorld(t)
	dead, alive := uniqueHash(t), uniqueHash(t)
	w.torrent(dead, false)
	w.torrent(alive, true)
	w.removed(dead)
	w.crossSeed(dead, alive)
	path := w.file("sonarr", dead, 11)

	sonarr := &fakeArr{}
	st, err := w.remover(map[string]Deleter{"sonarr": sonarr}).Run(w.ctx, false)
	if err != nil {
		t.Fatal(err)
	}

	if containsPath(st.Files, path) {
		t.Fatal("DELETED A FILE ANOTHER TRACKER IS STILL SEEDING")
	}
	if len(sonarr.deleted) != 0 {
		t.Errorf("deleted %v", sonarr.deleted)
	}
}

// The second hop. Several cross-seeds of one content connect through a middle
// member rather than directly, and a one-hop walk would miss the live one.
func TestKeepsFileWithLiveCrossSeedTwoHopsOut(t *testing.T) {
	w := newWorld(t)
	dead, middle, alive := uniqueHash(t), uniqueHash(t), uniqueHash(t)
	w.torrent(dead, false)
	w.torrent(middle, false)
	w.torrent(alive, true)
	w.removed(dead)
	w.crossSeed(dead, middle)
	w.crossSeed(middle, alive)
	path := w.file("sonarr", dead, 12)

	sonarr := &fakeArr{}
	st, err := w.remover(map[string]Deleter{"sonarr": sonarr}).Run(w.ctx, false)
	if err != nil {
		t.Fatal(err)
	}

	if containsPath(st.Files, path) {
		t.Fatal("two hops out, a tracker is still seeding this")
	}
}

// Edges are undirected. Which torrent cross-seed searched FROM is an accident
// of which was grabbed first, so a live sibling counts whichever end it is on.
func TestCrossSeedEdgeIsUndirected(t *testing.T) {
	w := newWorld(t)
	dead, alive := uniqueHash(t), uniqueHash(t)
	w.torrent(dead, false)
	w.torrent(alive, true)
	w.removed(dead)
	// Reversed: the LIVE torrent was the searchee, the dead one the candidate.
	w.crossSeed(alive, dead)
	path := w.file("sonarr", dead, 13)

	st, err := w.remover(map[string]Deleter{"sonarr": &fakeArr{}}).Run(w.ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if containsPath(st.Files, path) {
		t.Fatal("edge direction decided the outcome; it must not")
	}
}

// A torrent a client still holds is not orphaned, whatever torrent_removals
// says. A removal record plus a live client row means the delete failed, not
// that the content vanished.
func TestKeepsFileWhoseOwnTorrentIsStillHeld(t *testing.T) {
	w := newWorld(t)
	hash := uniqueHash(t)
	w.torrent(hash, true)
	w.removed(hash)
	path := w.file("sonarr", hash, 14)

	st, err := w.remover(map[string]Deleter{"sonarr": &fakeArr{}}).Run(w.ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if containsPath(st.Files, path) {
		t.Fatal("acted on a torrent the client still holds")
	}
}

// Lidarr is excluded by construction. Music arrives through Soulseek and beets
// as well as torrents, so an album with no live torrent is an ordinary album.
func TestLidarrIsNeverSelected(t *testing.T) {
	w := newWorld(t)
	hash := uniqueHash(t)
	w.torrent(hash, false)
	w.removed(hash)
	path := w.file("lidarr", hash, 15)

	lidarr := &fakeArr{}
	st, err := w.remover(map[string]Deleter{"lidarr": lidarr}).Run(w.ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if containsPath(st.Files, path) {
		t.Fatal("selected a Lidarr file")
	}
	if len(lidarr.deleted) != 0 {
		t.Fatalf("DELETED MUSIC: %v", lidarr.deleted)
	}
}

// A file the torrent-removal record does not cover is not a candidate. This is
// what keeps the pass scoped to what managarr itself deleted rather than to
// every file whose torrent happens to be absent.
func TestFileWithNoRemovalRecordIsIgnored(t *testing.T) {
	w := newWorld(t)
	hash := uniqueHash(t)
	w.torrent(hash, false)
	// No w.removed(hash).
	path := w.file("sonarr", hash, 16)

	st, err := w.remover(map[string]Deleter{"sonarr": &fakeArr{}}).Run(w.ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if containsPath(st.Files, path) {
		t.Fatal("selected a file managarr never deleted a torrent for")
	}
}

// A dry run finds exactly what a real run would and deletes none of it. This is
// the mode everything above this defaults to.
func TestDryRunFindsButDeletesNothing(t *testing.T) {
	w := newWorld(t)
	hash := uniqueHash(t)
	w.torrent(hash, false)
	w.removed(hash)
	path := w.file("sonarr", hash, 17)

	sonarr := &fakeArr{}
	st, err := w.remover(map[string]Deleter{"sonarr": sonarr}).Run(w.ctx, true)
	if err != nil {
		t.Fatal(err)
	}

	if !containsPath(st.Files, path) {
		t.Fatal("a dry run did not report the candidate")
	}
	if st.Deleted != 0 {
		t.Errorf("deleted = %d on a dry run", st.Deleted)
	}
	if len(sonarr.deleted) != 0 {
		t.Fatalf("A DRY RUN DELETED FILES: %v", sonarr.deleted)
	}
	if !st.DryRun {
		t.Error("stats do not record that this was a dry run")
	}
}

// A file whose service has no configured client is reported, not guessed at.
func TestUnroutableFileIsSurfaced(t *testing.T) {
	w := newWorld(t)
	hash := uniqueHash(t)
	w.torrent(hash, false)
	w.removed(hash)
	w.file("sonarr", hash, 18)

	// Only Radarr configured, so the Sonarr file has nowhere to go.
	st, err := w.remover(map[string]Deleter{"radarr": &fakeArr{}}).Run(w.ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.Unroutable == 0 {
		t.Error("unroutable file was not counted")
	}
	if st.Deleted != 0 {
		t.Errorf("deleted = %d", st.Deleted)
	}
}

// One *arr refusing must not stop the others. Each delete is independent, and
// aborting would make the run's effect depend on which candidate sorted first.
func TestFailureOnOneFileDoesNotStopTheRest(t *testing.T) {
	w := newWorld(t)
	dead1, dead2 := uniqueHash(t), uniqueHash(t)
	w.torrent(dead1, false)
	w.torrent(dead2, false)
	w.removed(dead1)
	w.removed(dead2)
	w.file("sonarr", dead1, 19)
	w.file("radarr", dead2, 20)

	sonarr := &fakeArr{err: errors.New("sonarr is unwell")}
	radarr := &fakeArr{}
	st, err := w.remover(map[string]Deleter{"sonarr": sonarr, "radarr": radarr}).Run(w.ctx, false)
	if err != nil {
		t.Fatalf("one refusal failed the whole run: %v", err)
	}

	if st.Failed != 1 {
		t.Errorf("failed = %d, want 1", st.Failed)
	}
	if st.Deleted != 1 {
		t.Errorf("deleted = %d, want the other one to have gone", st.Deleted)
	}
	if len(radarr.deleted) != 1 {
		t.Errorf("radarr deleted %v", radarr.deleted)
	}
}

// Nothing to do is not an error and touches no *arr.
//
// This test's world seeds nothing, so it also covers the early return: with no
// removal records at all, the pass must not go on to walk cross-seed edges.
func TestNoCandidatesCallsNothing(t *testing.T) {
	w := newWorld(t)
	sonarr := &fakeArr{}
	st, err := w.remover(map[string]Deleter{"sonarr": sonarr}).Run(w.ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(sonarr.deleted) != 0 {
		t.Errorf("deleted %v with no candidates", sonarr.deleted)
	}
	if st.Deleted != 0 || st.Failed != 0 || st.Unroutable != 0 {
		t.Errorf("stats = %+v, want all zero", st)
	}
}

func containsPath(files []Candidate, path string) bool {
	for _, f := range files {
		if f.Path == path {
			return true
		}
	}
	return false
}

// torrentFile records that a torrent holds a file with given byte identity.
func (w *world) torrentFile(hash string, inode, size int64) {
	w.t.Helper()
	err := w.db.WithContext(w.ctx).Exec(`
		INSERT INTO file_data (inode, size_bytes, last_observed_at)
		VALUES (?, ?, now())
		ON CONFLICT (inode, size_bytes) DO NOTHING`, inode, size).Error
	if err != nil {
		w.t.Fatal(err)
	}
	err = w.db.WithContext(w.ctx).Exec(`
		INSERT INTO torrent_files (torrent_infohash, path, file_data_inode, file_data_size_bytes)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (torrent_infohash, path) DO NOTHING`,
		hash, "/mnt/media/complete/tv/"+model.NewID().String()+".mkv", inode, size).Error
	if err != nil {
		w.t.Fatal(err)
	}
}

// fileWithBytes records a library file carrying a given byte identity.
func (w *world) fileWithBytes(service, hash string, arrFileID int, inode, size int64) string {
	w.t.Helper()
	err := w.db.WithContext(w.ctx).Exec(`
		INSERT INTO file_data (inode, size_bytes, last_observed_at)
		VALUES (?, ?, now())
		ON CONFLICT (inode, size_bytes) DO NOTHING`, inode, size).Error
	if err != nil {
		w.t.Fatal(err)
	}
	path := uniquePath(w.t)
	err = w.db.WithContext(w.ctx).Exec(`
		INSERT INTO library_files
		    (id, path, service_name, torrent_infohash, arr_file_id,
		     file_data_inode, file_data_size_bytes)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		model.NewID(), path, service, hash, arrFileID, inode, size).Error
	if err != nil {
		w.t.Fatal(err)
	}
	return path
}

// THE FIVE FALSE POSITIVES, reproduced.
//
// A library file whose named torrent was removed, with NO cross-seed edge to
// any survivor — but a different live torrent is seeding the very same bytes.
// The graph walk sees nothing and concludes orphan; the inode test sees the
// live provider.
//
// This is the real shape: 70.6% of cross-seed offers carry no source_infohash,
// so the providers of these files were invisible to the walk at any depth.
func TestLiveTorrentSeedingSameInodeIsNotAnOrphan(t *testing.T) {
	w := newWorld(t)
	dead, seeder := uniqueHash(t), uniqueHash(t)
	const inode, size = int64(368430), int64(3897998360)

	w.torrent(dead, false)
	w.removed(dead)
	w.torrent(seeder, true) // a live torrent, with NO offer edge to dead
	w.torrentFile(seeder, inode, size)
	path := w.fileWithBytes("sonarr", dead, 9859, inode, size)

	sonarr := &fakeArr{}
	st, err := w.remover(map[string]Deleter{"sonarr": sonarr}).Run(w.ctx, false)
	if err != nil {
		t.Fatal(err)
	}

	if containsPath(st.Files, path) {
		t.Fatalf("PROPOSED DELETING A FILE A LIVE TORRENT IS SEEDING: %+v", st.Files)
	}
	if len(sonarr.deleted) != 0 {
		t.Fatalf("DELETED IT: %v", sonarr.deleted)
	}
}

// Same bytes, but the torrent holding them is gone from the client: still an
// orphan. The test must key on LIVE torrents, not on the inode existing.
func TestInodeHeldOnlyByRemovedTorrentIsStillAnOrphan(t *testing.T) {
	w := newWorld(t)
	dead, alsoDead := uniqueHash(t), uniqueHash(t)
	const inode, size = int64(457338), int64(979248284)

	w.torrent(dead, false)
	w.removed(dead)
	w.torrent(alsoDead, false) // recorded, but no live client_torrent row
	w.torrentFile(alsoDead, inode, size)
	path := w.fileWithBytes("sonarr", dead, 13131, inode, size)

	st, err := w.remover(map[string]Deleter{"sonarr": &fakeArr{}}).Run(w.ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if !containsPath(st.Files, path) {
		t.Error("a genuinely orphaned file was spared by the inode test")
	}
}

// A different inode is a different file. Same size alone must not spare it —
// which is why the key is (inode, size) and not either half.
func TestDifferentInodeSameSizeIsStillAnOrphan(t *testing.T) {
	w := newWorld(t)
	dead, seeder := uniqueHash(t), uniqueHash(t)
	const size = int64(1234567)

	w.torrent(dead, false)
	w.removed(dead)
	w.torrent(seeder, true)
	w.torrentFile(seeder, 111111, size)
	path := w.fileWithBytes("sonarr", dead, 4242, 222222, size)

	st, err := w.remover(map[string]Deleter{"sonarr": &fakeArr{}}).Run(w.ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if !containsPath(st.Files, path) {
		t.Error("an unrelated file sharing only a size was spared")
	}
}

// A library file the stat pass has not reached has inode 0. That is a missing
// measurement, not a match against every other unmeasured file.
func TestUnstattedFileIsNotSparedByInodeZero(t *testing.T) {
	w := newWorld(t)
	dead, seeder := uniqueHash(t), uniqueHash(t)

	w.torrent(dead, false)
	w.removed(dead)
	w.torrent(seeder, true)
	// The seeder's file is also unmeasured: both would be inode 0.
	path := w.file("sonarr", dead, 7777)

	st, err := w.remover(map[string]Deleter{"sonarr": &fakeArr{}}).Run(w.ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if !containsPath(st.Files, path) {
		t.Error("inode 0 was treated as a match and spared the file")
	}
}
