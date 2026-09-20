package tag

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

// Against a real PostgreSQL, like the rest of managarr's data tests.
//
//	MANAGARR_TEST_DSN=postgres://managarr@host:5432/managarr_test?sslmode=disable go test ./...
const testDSNEnv = "MANAGARR_TEST_DSN"

const (
	linksDir = "/mnt/media/cross-seed/links"
	packsDir = "/mnt/media/complete/seasonpacks"
)

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

// fakeSetter records what was written, and can refuse.
type fakeSetter struct {
	calls []setCall
	err   error
	// registry is what the client reports as registered, independent of use.
	registry []string
	deleted  []string
}

type setCall struct {
	hashes []string
	tags   []string
}

func (f *fakeSetter) SetTags(_ context.Context, hashes, tags []string) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, setCall{hashes: hashes, tags: tags})
	return nil
}

// registry is the client's tag list, which outlives the torrents carrying it.
func (f *fakeSetter) Tags(_ context.Context) ([]string, error) { return f.registry, nil }

func (f *fakeSetter) DeleteTags(_ context.Context, tags []string) error {
	f.deleted = append(f.deleted, tags...)
	return nil
}

func (f *fakeSetter) tagsFor(hash string) ([]string, bool) {
	for _, c := range f.calls {
		for _, h := range c.hashes {
			if h == hash {
				return c.tags, true
			}
		}
	}
	return nil, false
}

func uniqueHash(t *testing.T) string {
	t.Helper()
	return strings.ReplaceAll(model.NewID().String(), "-", "") + "00000000"
}

func clientName(t *testing.T) string {
	t.Helper()
	id := strings.ReplaceAll(model.NewID().String(), "-", "")
	return "qbt-" + t.Name() + "-" + id[len(id)-8:]
}

// world seeds one torrent's worth of facts.
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

// torrent records a torrent the client holds, with its current tags and path.
func (w *world) torrent(hash, tags, contentPath string) {
	w.t.Helper()
	err := w.db.WithContext(w.ctx).Exec(`
		INSERT INTO torrents (infohash) VALUES (?)
		ON CONFLICT (infohash) DO NOTHING`, hash).Error
	if err != nil {
		w.t.Fatal(err)
	}
	err = w.db.WithContext(w.ctx).Exec(`
		INSERT INTO client_torrents (client_name, torrent_infohash, tags, content_path)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (client_name, torrent_infohash) DO UPDATE
		   SET tags = EXCLUDED.tags, content_path = EXCLUDED.content_path`,
		w.client, hash, tags, contentPath).Error
	if err != nil {
		w.t.Fatal(err)
	}
}

// tracker attaches an announce host, optionally with an error message.
func (w *world) tracker(hash, host, message string, code int) {
	w.t.Helper()
	err := w.db.WithContext(w.ctx).Exec(`
		INSERT INTO trackers (announce_host) VALUES (?)
		ON CONFLICT (announce_host) DO NOTHING`, host).Error
	if err != nil {
		w.t.Fatal(err)
	}
	err = w.db.WithContext(w.ctx).Exec(`
		INSERT INTO torrent_trackers (torrent_infohash, tracker_announce_host, status, status_code, message, url)
		VALUES (?, ?, 'x', ?, ?, ?)
		ON CONFLICT (torrent_infohash, tracker_announce_host) DO UPDATE
		   SET status_code = EXCLUDED.status_code, message = EXCLUDED.message`,
		hash, host, code, message, "https://"+host+"/announce").Error
	if err != nil {
		w.t.Fatal(err)
	}
}

// imported records that an *arr imported a file from this torrent.
func (w *world) imported(hash string) {
	w.t.Helper()
	err := w.db.WithContext(w.ctx).Exec(`
		INSERT INTO library_files (id, path, service_name, torrent_infohash)
		VALUES (?, ?, 'sonarr', ?)`,
		model.NewID(), "/mnt/media/library/tv/"+w.t.Name()+"/"+model.NewID().String()+".mkv", hash).Error
	if err != nil {
		w.t.Fatal(err)
	}
}

// packSource records that a pack was assembled from an episode torrent.
func (w *world) packSource(pack, source string) {
	w.t.Helper()
	err := w.db.WithContext(w.ctx).Exec(`
		INSERT INTO pack_sources (pack_infohash, source_infohash, via, matched_name, observed_at)
		VALUES (?, ?, 'pack_contents', 'x', now())
		ON CONFLICT (pack_infohash, source_infohash) DO NOTHING`,
		pack, source).Error
	if err != nil {
		w.t.Fatal(err)
	}
}

// pairing records cross-seed's statement that candidate came from source.
func (w *world) pairing(source, candidate string) {
	w.t.Helper()
	err := w.db.WithContext(w.ctx).Exec(`
		INSERT INTO offers (id, kind, source_ref, source_infohash, infohash, discovered_at)
		VALUES (?, 'cross_seed_candidate', ?, ?, ?, now())`,
		model.NewID(), model.NewID().String(), source, candidate).Error
	if err != nil {
		w.t.Fatal(err)
	}
}

func (w *world) tagger(c Setter, trackerTags map[string]string) *Tagger {
	return New(w.db, c, discardLogger(), Config{
		ClientName:  w.client,
		TrackerTags: trackerTags,
		LinksDir:    linksDir,
		PacksDir:    packsDir,
	})
}

// One tracker announcing under two hostnames must produce ONE tag.
//
// The reason the mapping is configuration rather than derivation:
// tracker.torrentleech.org and tracker.tleechreload.org are the same site and
// share no derivable common part.
func TestTwoAnnounceHostsOneTrackerTag(t *testing.T) {
	w := newWorld(t)
	hash := uniqueHash(t)
	w.torrent(hash, "", "/mnt/media/complete/tv/Show.mkv")
	w.tracker(hash, "tracker.torrentleech.org", "", 2)
	w.tracker(hash, "tracker.tleechreload.org", "", 2)

	f := &fakeSetter{}
	_, err := w.tagger(f, map[string]string{
		"tracker.torrentleech.org": "TL",
		"tracker.tleechreload.org": "TL",
	}).Run(w.ctx, false)
	if err != nil {
		t.Fatal(err)
	}

	tags, ok := f.tagsFor(hash)
	if !ok {
		t.Fatal("torrent was not tagged")
	}
	if n := countPrefix(tags, "tracker::"); n != 1 {
		t.Errorf("got %d tracker tags in %v, want 1", n, tags)
	}
	if !contains(tags, "tracker::TL") {
		t.Errorf("tags = %v, want tracker::TL", tags)
	}
}

// An unmapped host tags with the hostname rather than vanishing, so a missing
// mapping is visible in the client instead of silently producing no tag.
func TestUnmappedTrackerUsesHostname(t *testing.T) {
	w := newWorld(t)
	hash := uniqueHash(t)
	w.torrent(hash, "", "/mnt/media/complete/tv/Show.mkv")
	w.tracker(hash, "brand.new.tracker.example", "", 2)

	f := &fakeSetter{}
	if _, err := w.tagger(f, nil).Run(w.ctx, false); err != nil {
		t.Fatal(err)
	}
	tags, _ := f.tagsFor(hash)
	if !contains(tags, "tracker::brand.new.tracker.example") {
		t.Errorf("tags = %v, want the raw hostname", tags)
	}
}

// library:: comes from the import record, which is the fact hardlink counting
// was guessing at.
func TestLinkedAndUnlinked(t *testing.T) {
	w := newWorld(t)
	linked, bare := uniqueHash(t), uniqueHash(t)
	w.torrent(linked, "", "/mnt/media/complete/tv/A.mkv")
	w.torrent(bare, "", "/mnt/media/complete/tv/B.mkv")
	w.imported(linked)

	f := &fakeSetter{}
	if _, err := w.tagger(f, nil).Run(w.ctx, false); err != nil {
		t.Fatal(err)
	}

	got, _ := f.tagsFor(linked)
	if !contains(got, "library::linked") {
		t.Errorf("imported torrent tags = %v", got)
	}
	got, _ = f.tagsFor(bare)
	if !contains(got, "library::unlinked") {
		t.Errorf("un-imported torrent tags = %v", got)
	}
}

// state::unregistered matches on the MESSAGE, not the status code: code 5 is a
// generic tracker error that covers notices from trackers working perfectly.
func TestUnregisteredMatchesMessageNotCode(t *testing.T) {
	w := newWorld(t)
	dead, healthy := uniqueHash(t), uniqueHash(t)
	w.torrent(dead, "", "/mnt/media/complete/tv/A.mkv")
	w.torrent(healthy, "", "/mnt/media/complete/tv/B.mkv")
	w.tracker(dead, "a.example", "unregistered torrent", 4)
	w.tracker(healthy, "b.example", "You already have 3 peers on this torrent. Ignoring.", 4)

	f := &fakeSetter{}
	if _, err := w.tagger(f, nil).Run(w.ctx, false); err != nil {
		t.Fatal(err)
	}

	got, _ := f.tagsFor(dead)
	if !contains(got, "state::unregistered") {
		t.Errorf("dead torrent tags = %v", got)
	}
	got, _ = f.tagsFor(healthy)
	if contains(got, "state::unregistered") {
		t.Errorf("MARKED A HEALTHY TORRENT UNREGISTERED: %v", got)
	}
}

// xseed::inject is structural — content under the link directory was put there
// by cross-seed.
func TestInjectComesFromThePath(t *testing.T) {
	w := newWorld(t)
	inject, original := uniqueHash(t), uniqueHash(t)
	w.torrent(inject, "", linksDir+"/YU-Scene/Show.S01E08.mkv")
	w.torrent(original, "", "/mnt/media/complete/tv/Show.S01E08.mkv")

	f := &fakeSetter{}
	if _, err := w.tagger(f, nil).Run(w.ctx, false); err != nil {
		t.Fatal(err)
	}

	got, _ := f.tagsFor(inject)
	if !contains(got, "xseed::inject") {
		t.Errorf("inject tags = %v", got)
	}
	got, _ = f.tagsFor(original)
	if contains(got, "xseed::inject") {
		t.Errorf("original tagged as an inject: %v", got)
	}
}

// xseed::source marks the torrent an inject actually came from.
func TestSourceOfRealInject(t *testing.T) {
	w := newWorld(t)
	source, inject := uniqueHash(t), uniqueHash(t)
	w.torrent(source, "", "/mnt/media/complete/tv/Show.mkv")
	w.torrent(inject, "", linksDir+"/YU-Scene/Show.mkv")
	w.pairing(source, inject)

	f := &fakeSetter{}
	if _, err := w.tagger(f, nil).Run(w.ctx, false); err != nil {
		t.Fatal(err)
	}

	got, _ := f.tagsFor(source)
	if !contains(got, "xseed::source") {
		t.Errorf("source tags = %v", got)
	}
}

// A pairing whose candidate was never injected must NOT mark a source.
//
// cross-seed proposes candidates it rejects, and it matches torrents already
// held. Counting every proposal marked 1514 torrents where only 328 had
// produced anything.
func TestProposedButNotInjectedIsNotASource(t *testing.T) {
	w := newWorld(t)
	source, proposed := uniqueHash(t), uniqueHash(t)
	w.torrent(source, "", "/mnt/media/complete/tv/Show.mkv")
	// The candidate exists as an offer but is not in the client's link dir, so
	// it was never injected.
	w.pairing(source, proposed)

	f := &fakeSetter{}
	if _, err := w.tagger(f, nil).Run(w.ctx, false); err != nil {
		t.Fatal(err)
	}

	got, _ := f.tagsFor(source)
	if contains(got, "xseed::source") {
		t.Errorf("a rejected proposal marked a source: %v", got)
	}
}

// Tags are REPLACED, so anything else the torrent carried is gone.
func TestExistingTagsAreReplaced(t *testing.T) {
	w := newWorld(t)
	hash := uniqueHash(t)
	w.torrent(hash, "cross-seed, cleanuparr-unlinked, noHL", "/mnt/media/complete/tv/Show.mkv")
	w.tracker(hash, "seedpool.org", "", 2)

	f := &fakeSetter{}
	if _, err := w.tagger(f, map[string]string{"seedpool.org": "SP"}).Run(w.ctx, false); err != nil {
		t.Fatal(err)
	}

	tags, _ := f.tagsFor(hash)
	for _, stale := range []string{"cross-seed", "cleanuparr-unlinked", "noHL"} {
		if contains(tags, stale) {
			t.Errorf("stale tag %q survived: %v", stale, tags)
		}
	}
	if !contains(tags, "tracker::SP") {
		t.Errorf("tags = %v", tags)
	}
}

// A torrent already carrying exactly the right tags is not rewritten.
//
// The client holds ~7600 torrents whose tags do not change between runs;
// writing unconditionally would be thousands of pointless mutations an hour.
func TestUnchangedTorrentIsNotWritten(t *testing.T) {
	w := newWorld(t)
	hash := uniqueHash(t)
	// Exactly what the tagger will compute, in the client's own rendering:
	// comma-space separated and unsorted.
	w.torrent(hash, "library::unlinked, tracker::SP", "/mnt/media/complete/tv/Show.mkv")
	w.tracker(hash, "seedpool.org", "", 2)

	f := &fakeSetter{}
	st, err := w.tagger(f, map[string]string{"seedpool.org": "SP"}).Run(w.ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, written := f.tagsFor(hash); written {
		t.Errorf("rewrote a torrent whose tags already matched; calls = %+v", f.calls)
	}
	if st.Changed != 0 {
		t.Errorf("changed = %d, want 0", st.Changed)
	}
}

// A dry run computes everything and writes nothing.
func TestDryRunWritesNothing(t *testing.T) {
	w := newWorld(t)
	hash := uniqueHash(t)
	w.torrent(hash, "", "/mnt/media/complete/tv/Show.mkv")
	w.tracker(hash, "seedpool.org", "", 2)

	f := &fakeSetter{}
	st, err := w.tagger(f, map[string]string{"seedpool.org": "SP"}).Run(w.ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 0 {
		t.Fatalf("A DRY RUN WROTE TAGS: %+v", f.calls)
	}
	if st.Changed == 0 {
		t.Error("a dry run reported nothing to change")
	}
	if st.Applied != 0 {
		t.Errorf("applied = %d on a dry run", st.Applied)
	}
	if !st.DryRun {
		t.Error("stats do not record that this was a dry run")
	}
}

// Torrents wanting identical tags are written in one call, not one each.
func TestIdenticalTagSetsBatchIntoOneCall(t *testing.T) {
	w := newWorld(t)
	a, b := uniqueHash(t), uniqueHash(t)
	w.torrent(a, "", "/mnt/media/complete/tv/A.mkv")
	w.torrent(b, "", "/mnt/media/complete/tv/B.mkv")
	w.tracker(a, "seedpool.org", "", 2)
	w.tracker(b, "seedpool.org", "", 2)

	f := &fakeSetter{}
	st, err := w.tagger(f, map[string]string{"seedpool.org": "SP"}).Run(w.ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.Calls != 1 {
		t.Errorf("calls = %d, want 1 for two identical tag sets", st.Calls)
	}
	if len(f.calls) != 1 || len(f.calls[0].hashes) != 2 {
		t.Errorf("calls = %+v", f.calls)
	}
}

// A refusal surfaces rather than reporting success.
func TestClientFailureIsReported(t *testing.T) {
	w := newWorld(t)
	hash := uniqueHash(t)
	w.torrent(hash, "", "/mnt/media/complete/tv/Show.mkv")

	f := &fakeSetter{err: errors.New("qbittorrent is unwell")}
	if _, err := w.tagger(f, nil).Run(w.ctx, false); err == nil {
		t.Fatal("expected an error when the client refuses")
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"b, a":            "a,b",
		"a,b":             "a,b",
		"  a ,  b  ":      "a,b",
		"":                "",
		"a, a":            "a",
		"library::linked": "library::linked",
	}
	for in, want := range cases {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func countPrefix(s []string, prefix string) int {
	n := 0
	for _, x := range s {
		if strings.HasPrefix(x, prefix) {
			n++
		}
	}
	return n
}

// pack::assembled marks a season pack seasonpackarr built from episodes already
// in the client, which is a third origin — neither grabbed nor cross-seeded.
func TestAssembledPackIsTagged(t *testing.T) {
	w := newWorld(t)
	pack, grabbed := uniqueHash(t), uniqueHash(t)
	w.torrent(pack, "", packsDir+"/Show.S01.1080p.WEB-DL")
	w.torrent(grabbed, "", "/mnt/media/complete/tv/Show.S02.1080p.WEB-DL")

	f := &fakeSetter{}
	if _, err := w.tagger(f, nil).Run(w.ctx, false); err != nil {
		t.Fatal(err)
	}

	got, _ := f.tagsFor(pack)
	if !contains(got, "pack::assembled") {
		t.Errorf("assembled pack tags = %v", got)
	}
	got, _ = f.tagsFor(grabbed)
	if contains(got, "pack::assembled") {
		t.Errorf("a grabbed pack was tagged as assembled: %v", got)
	}
}

// A sibling directory sharing a prefix must not match.
func TestUnderRequiresASeparator(t *testing.T) {
	p := "/mnt/media/complete/seasonpacks-old/Show.S01"
	if under(&p, packsDir) {
		t.Errorf("%q matched root %q", p, packsDir)
	}
	q := packsDir + "/Show.S01"
	if !under(&q, packsDir) {
		t.Errorf("%q did not match root %q", q, packsDir)
	}
	if under(nil, packsDir) {
		t.Error("nil content path matched")
	}
	if under(&q, "") {
		t.Error("empty root matched")
	}
}

// pack::feeds marks an episode torrent an assembled pack was built from.
//
// The tag that matters most for deletion: a pack and its episodes are the same
// bytes under two names, so removing either side's data breaks the survivor.
func TestPackSourceIsTagged(t *testing.T) {
	w := newWorld(t)
	episode, pack, unrelated := uniqueHash(t), uniqueHash(t), uniqueHash(t)
	w.torrent(episode, "", "/mnt/media/complete/tv/Show.S01E01.mkv")
	w.torrent(pack, "", packsDir+"/Show.S01.1080p.WEB-DL")
	w.torrent(unrelated, "", "/mnt/media/complete/tv/Other.S01E01.mkv")
	w.packSource(pack, episode)

	f := &fakeSetter{}
	if _, err := w.tagger(f, nil).Run(w.ctx, false); err != nil {
		t.Fatal(err)
	}

	got, _ := f.tagsFor(episode)
	if !contains(got, "pack::feeds") {
		t.Errorf("episode feeding a pack: tags = %v", got)
	}
	// The pack itself is assembled, not a feeder.
	got, _ = f.tagsFor(pack)
	if !contains(got, "pack::assembled") {
		t.Errorf("pack tags = %v", got)
	}
	if contains(got, "pack::feeds") {
		t.Errorf("pack tagged as feeding itself: %v", got)
	}
	got, _ = f.tagsFor(unrelated)
	if contains(got, "pack::feeds") {
		t.Errorf("unrelated torrent tagged as a pack source: %v", got)
	}
}

// queued records that a torrent sits in an *arr's queue.
func (w *world) queued(hash, service, trackedStatus string) {
	w.t.Helper()
	err := w.db.WithContext(w.ctx).Exec(`
		INSERT INTO services (name, kind) VALUES (?, 'sonarr')
		ON CONFLICT (name) DO NOTHING`, service).Error
	if err != nil {
		w.t.Fatal(err)
	}
	err = w.db.WithContext(w.ctx).Exec(`
		INSERT INTO queue_items (service_name, external_id, torrent_infohash, tracked_status, observed_at)
		VALUES (?, ?, ?, ?, now())
		ON CONFLICT (service_name, external_id) DO UPDATE
		   SET tracked_status = EXCLUDED.tracked_status`,
		service, model.NewID().String(), hash, trackedStatus).Error
	if err != nil {
		w.t.Fatal(err)
	}
}

// The case that motivated the split: a download completed a minute ago is not
// the same as one nothing will ever do anything with.
//
// Before the queue was consulted both read as library::unlinked, which invites
// a future prune rule to treat an in-flight import as abandoned.
func TestQueuedIsNotUnlinked(t *testing.T) {
	w := newWorld(t)
	fresh := uniqueHash(t)
	w.torrent(fresh, "", "/mnt/media/complete/tv/Just.Downloaded.mkv")
	w.queued(fresh, "sonarr-"+t.Name(), "ok/downloading")

	f := &fakeSetter{}
	if _, err := w.tagger(f, nil).Run(w.ctx, false); err != nil {
		t.Fatal(err)
	}

	got, _ := f.tagsFor(fresh)
	if !contains(got, "library::queued") {
		t.Errorf("tags = %v, want library::queued", got)
	}
	if contains(got, "library::unlinked") {
		t.Error("an in-flight import was reported as unlinked")
	}
}

// A queue the *arr refuses to advance is its own state, not "queued".
func TestImportBlockedIsBlocked(t *testing.T) {
	w := newWorld(t)
	stuck := uniqueHash(t)
	w.torrent(stuck, "", "/mnt/media/complete/tv/Stuck.mkv")
	w.queued(stuck, "sonarr-"+t.Name(), "warning/importBlocked")

	f := &fakeSetter{}
	if _, err := w.tagger(f, nil).Run(w.ctx, false); err != nil {
		t.Fatal(err)
	}

	got, _ := f.tagsFor(stuck)
	if !contains(got, "library::blocked") {
		t.Errorf("tags = %v, want library::blocked", got)
	}
	for _, never := range []string{"library::queued", "library::unlinked", "library::linked"} {
		if contains(got, never) {
			t.Errorf("blocked torrent also carried %s: %v", never, got)
		}
	}
}

// importFailed is blocked too — both need a person.
func TestImportFailedIsBlocked(t *testing.T) {
	w := newWorld(t)
	failed := uniqueHash(t)
	w.torrent(failed, "", "/mnt/media/complete/tv/Failed.mkv")
	w.queued(failed, "sonarr-"+t.Name(), "warning/importFailed")

	f := &fakeSetter{}
	if _, err := w.tagger(f, nil).Run(w.ctx, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.tagsFor(failed); !contains(got, "library::blocked") {
		t.Errorf("tags = %v", got)
	}
}

// Imported wins over whatever the queue still says: it reached the library.
func TestLinkedBeatsQueued(t *testing.T) {
	w := newWorld(t)
	done := uniqueHash(t)
	w.torrent(done, "", "/mnt/media/complete/tv/Done.mkv")
	w.imported(done)
	w.queued(done, "sonarr-"+t.Name(), "warning/importBlocked")

	f := &fakeSetter{}
	if _, err := w.tagger(f, nil).Run(w.ctx, false); err != nil {
		t.Fatal(err)
	}
	got, _ := f.tagsFor(done)
	if !contains(got, "library::linked") {
		t.Errorf("tags = %v, want library::linked", got)
	}
	if contains(got, "library::blocked") {
		t.Errorf("imported torrent also tagged blocked: %v", got)
	}
}

// In no queue and in no library is the only thing unlinked should now mean.
func TestUnlinkedMeansNothingPending(t *testing.T) {
	w := newWorld(t)
	idle := uniqueHash(t)
	w.torrent(idle, "", "/mnt/media/complete/tv/Idle.mkv")

	f := &fakeSetter{}
	if _, err := w.tagger(f, nil).Run(w.ctx, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.tagsFor(idle); !contains(got, "library::unlinked") {
		t.Errorf("tags = %v", got)
	}
}

// Two *arrs queueing one torrent: the worse state wins. A blocked import is a
// fact about the torrent, not about which service was asked.
func TestWorstQueueStateWins(t *testing.T) {
	w := newWorld(t)
	both := uniqueHash(t)
	w.torrent(both, "", "/mnt/media/complete/tv/Both.mkv")
	w.queued(both, "sonarr-"+t.Name(), "ok/downloading")
	w.queued(both, "radarr-"+t.Name(), "warning/importBlocked")

	f := &fakeSetter{}
	if _, err := w.tagger(f, nil).Run(w.ctx, false); err != nil {
		t.Fatal(err)
	}
	got, _ := f.tagsFor(both)
	if !contains(got, "library::blocked") {
		t.Errorf("tags = %v, want the worse state", got)
	}
}

func TestTrackedState(t *testing.T) {
	cases := map[string]string{
		"warning/importBlocked": "importBlocked",
		"ok/downloading":        "downloading",
		"importPending":         "importPending",
		"":                      "",
	}
	for in, want := range cases {
		if got := trackedState(in); got != want {
			t.Errorf("trackedState(%q) = %q, want %q", in, got, want)
		}
	}
}

// The bug this was written for: removing a tag from every torrent does NOT
// unregister it.
//
// qBittorrent keeps a tag in its list after the last torrent lets it go, so
// seventeen tags from two retired tools stayed in the sidebar looking current
// long after nothing carried them. Only the registry shows this; the torrents
// all looked correct.
func TestUnusedTagsAreUnregistered(t *testing.T) {
	w := newWorld(t)
	hash := uniqueHash(t)
	w.torrent(hash, "cross-seed", "/mnt/media/complete/tv/Show.mkv")
	w.tracker(hash, "seedpool.org", "", 2)

	f := &fakeSetter{registry: []string{
		"cross-seed", "cleanuparr-unlinked", "noHL", "tracker-TL", "tv",
		"library::unlinked", "tracker::SP",
	}}
	st, err := w.tagger(f, map[string]string{"seedpool.org": "SP"}).Run(w.ctx, false)
	if err != nil {
		t.Fatal(err)
	}

	for _, dead := range []string{"cross-seed", "cleanuparr-unlinked", "noHL", "tracker-TL", "tv"} {
		if !contains(f.deleted, dead) {
			t.Errorf("%q left registered; it is what the sidebar shows", dead)
		}
	}
	// What the pass just assigned must survive.
	for _, live := range []string{"library::unlinked", "tracker::SP"} {
		if contains(f.deleted, live) {
			t.Errorf("unregistered %q, which torrents now carry", live)
		}
	}
	if len(st.Pruned) == 0 {
		t.Error("pruned tags were not reported")
	}
}

// A dry run must not touch the registry either.
func TestDryRunDoesNotPruneRegistry(t *testing.T) {
	w := newWorld(t)
	hash := uniqueHash(t)
	w.torrent(hash, "cross-seed", "/mnt/media/complete/tv/Show.mkv")

	f := &fakeSetter{registry: []string{"cross-seed", "noHL"}}
	st, err := w.tagger(f, nil).Run(w.ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.deleted) != 0 {
		t.Errorf("A DRY RUN UNREGISTERED TAGS: %v", f.deleted)
	}
	if len(st.Pruned) != 0 {
		t.Errorf("dry run reported pruning %v", st.Pruned)
	}
}

// A registry already holding only what is in use needs no call at all.
func TestCleanRegistryIsLeftAlone(t *testing.T) {
	w := newWorld(t)
	hash := uniqueHash(t)
	w.torrent(hash, "", "/mnt/media/complete/tv/Show.mkv")
	w.tracker(hash, "seedpool.org", "", 2)

	f := &fakeSetter{registry: []string{"library::unlinked", "tracker::SP"}}
	if _, err := w.tagger(f, map[string]string{"seedpool.org": "SP"}).Run(w.ctx, false); err != nil {
		t.Fatal(err)
	}
	if len(f.deleted) != 0 {
		t.Errorf("deleted %v from a registry with nothing dead in it", f.deleted)
	}
}
