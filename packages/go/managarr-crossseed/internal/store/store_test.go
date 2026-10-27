package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// schema is cross-seed's, reduced to the columns this package names.
//
// Written out rather than fixtured from a real library on purpose: what these
// tests defend is the table and column names in the queries. A cross-seed
// release that renames one has to fail here, loudly, rather than return zero
// decisions from a sidecar that looks perfectly healthy.
const schema = `
CREATE TABLE searchee (
	id INTEGER PRIMARY KEY,
	name TEXT
);
CREATE TABLE decision (
	id INTEGER PRIMARY KEY,
	searchee_id INTEGER,
	guid TEXT,
	info_hash TEXT,
	decision TEXT,
	first_seen INTEGER,
	last_seen INTEGER
);
CREATE TABLE client_searchee (
	client_host TEXT,
	info_hash TEXT,
	name TEXT,
	title TEXT,
	files TEXT,
	length INTEGER,
	save_path TEXT,
	category TEXT,
	tags TEXT,
	trackers TEXT,
	PRIMARY KEY (client_host, info_hash)
);
CREATE TABLE ensemble (
	client_host TEXT,
	path TEXT,
	info_hash TEXT,
	ensemble TEXT,
	element TEXT,
	PRIMARY KEY (client_host, path)
);`

// newDB writes a cross-seed-shaped database and returns its path.
func newDB(t *testing.T, exec ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cross-seed.db")

	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, stmt := range append([]string{schema}, exec...) {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %.60s: %v", stmt, err)
		}
	}
	return path
}

// seedDecisions writes n decisions with ascending first_seen, each on its own
// searchee.
func seedDecisions(n int) string {
	out := ""
	for i := range n {
		out += fmt.Sprintf(
			"INSERT INTO searchee (id, name) VALUES (%d, 'Release.%d');"+
				"INSERT INTO decision (id, searchee_id, guid, info_hash, decision, first_seen, last_seen)"+
				" VALUES (%d, %d, 'https://x/%d', '%040x', 'SIZE_MISMATCH', %d, %d);",
			i, i, i, i, i, i, 1700000000000+int64(i)*1000, 1700000000000+int64(i)*1000)
	}
	return out
}

func TestDecisions(t *testing.T) {
	path := newDB(t,
		`INSERT INTO searchee (id, name) VALUES (1, 'Some.Release.2019.1080p.BluRay-GRP')`,
		`INSERT INTO decision (id, searchee_id, guid, info_hash, decision, first_seen, last_seen)
		 VALUES (7, 1, 'https://tracker.example/dl/1?rss=SECRET', 'ABC123', 'MATCH',
		         1700000000000, 1700003600000)`,
	)

	got, err := New(path).Decisions(context.Background(), Cursor{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d decisions, want 1", len(got))
	}

	d := got[0].Decision
	if d.SearcheeName != "Some.Release.2019.1080p.BluRay-GRP" ||
		d.InfoHash != "ABC123" || d.Decision != "MATCH" ||
		d.GUID != "https://tracker.example/dl/1?rss=SECRET" {
		t.Errorf("decision = %+v", d)
	}

	// cross-seed stores epoch MILLISECONDS. Reading them as seconds would put
	// every decision fifty thousand years out and still parse fine.
	if !d.FirstSeen.Equal(time.UnixMilli(1700000000000).UTC()) {
		t.Errorf("FirstSeen = %v", d.FirstSeen)
	}
	if !d.LastSeen.Equal(time.UnixMilli(1700003600000).UTC()) {
		t.Errorf("LastSeen = %v", d.LastSeen)
	}

	// The cursor carries the row id, not just the timestamp.
	if got[0].Cursor != (Cursor{FirstSeenMS: 1700000000000, ID: 7}) {
		t.Errorf("cursor = %+v", got[0].Cursor)
	}
}

// Every column but first_seen is nullable in practice, and a null must read as
// empty rather than failing the scan and taking the whole page with it.
func TestDecisionsNullColumns(t *testing.T) {
	path := newDB(t,
		`INSERT INTO searchee (id, name) VALUES (1, 'Release')`,
		`INSERT INTO decision (id, searchee_id, guid, info_hash, decision, first_seen, last_seen)
		 VALUES (1, 1, NULL, NULL, NULL, 1700000000000, NULL)`,
	)

	got, err := New(path).Decisions(context.Background(), Cursor{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d decisions, want 1", len(got))
	}
	d := got[0].Decision
	if d.InfoHash != "" || d.GUID != "" || d.Decision != "" {
		t.Errorf("nulls did not read as empty: %+v", d)
	}
	// A null last_seen falls back to first_seen, so a consumer ordering by it
	// never sees the zero time.
	if !d.LastSeen.Equal(d.FirstSeen) {
		t.Errorf("LastSeen = %v, want the FirstSeen fallback %v", d.LastSeen, d.FirstSeen)
	}
}

// A decision whose searchee row is gone is dropped by the join. Intended:
// SearcheeName is the only handle managarr correlates on, so a row without one
// carries nothing usable.
func TestDecisionsOrphanDropped(t *testing.T) {
	path := newDB(t,
		`INSERT INTO decision (id, searchee_id, first_seen) VALUES (1, 999, 1700000000000)`,
	)

	got, err := New(path).Decisions(context.Background(), Cursor{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %d decisions, want the orphan dropped", len(got))
	}
}

func TestDecisionsKeysetPaging(t *testing.T) {
	st := New(newDB(t, seedDecisions(25)))
	ctx := context.Background()

	first, err := st.Decisions(ctx, Cursor{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 10 {
		t.Fatalf("first page = %d rows, want 10", len(first))
	}
	if first[0].SearcheeName != "Release.0" {
		t.Errorf("first row = %q, want Release.0", first[0].SearcheeName)
	}

	// Resume from the last cursor: the next page must start immediately
	// after, with nothing repeated and nothing skipped.
	second, err := st.Decisions(ctx, first[9].Cursor, 10)
	if err != nil {
		t.Fatal(err)
	}
	if second[0].SearcheeName != "Release.10" {
		t.Errorf("resumed at %q, want Release.10", second[0].SearcheeName)
	}

	last, err := st.Decisions(ctx, second[9].Cursor, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(last) != 5 || last[4].SearcheeName != "Release.24" {
		t.Errorf("final page = %d rows ending %q", len(last), last[len(last)-1].SearcheeName)
	}
}

// The whole reason the cursor carries an id. cross-seed writes a verdict per
// candidate, so one search lands several decisions in the same millisecond. A
// timestamp-only cursor would either re-read that group forever or step past
// part of it.
func TestDecisionsTiedTimestamps(t *testing.T) {
	const ms = 1700000000000
	rows := ""
	for i := 1; i <= 5; i++ {
		rows += fmt.Sprintf(
			"INSERT INTO searchee (id, name) VALUES (%d, 'Tied.%d');"+
				"INSERT INTO decision (id, searchee_id, decision, first_seen, last_seen)"+
				" VALUES (%d, %d, 'SIZE_MISMATCH', %d, %d);",
			i, i, i, i, ms, ms)
	}
	st := New(newDB(t, rows))
	ctx := context.Background()

	page, err := st.Decisions(ctx, Cursor{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 {
		t.Fatalf("first page = %d rows, want 2", len(page))
	}

	// Walk the rest with the cursor and check every tied row appears once.
	seen := map[string]int{}
	for _, r := range page {
		seen[r.SearcheeName]++
	}
	cursor := page[len(page)-1].Cursor
	for range 10 {
		next, err := st.Decisions(ctx, cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(next) == 0 {
			break
		}
		for _, r := range next {
			seen[r.SearcheeName]++
		}
		cursor = next[len(next)-1].Cursor
	}

	if len(seen) != 5 {
		t.Errorf("saw %d distinct rows, want 5: %v", len(seen), seen)
	}
	for name, n := range seen {
		if n != 1 {
			t.Errorf("%s appeared %d times, want once", name, n)
		}
	}
}

func TestDecisionsPastEnd(t *testing.T) {
	st := New(newDB(t, seedDecisions(3)))

	got, err := st.Decisions(context.Background(),
		Cursor{FirstSeenMS: 1<<62 - 1, ID: 1<<62 - 1}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %d rows past the end, want 0", len(got))
	}
}

func TestCursorRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   Cursor
		want string
	}{
		{"zero is empty", Cursor{}, ""},
		{"encoded", Cursor{FirstSeenMS: 1700000000000, ID: 42}, "1700000000000:42"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.String(); got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
			back, err := ParseCursor(tc.want)
			if err != nil {
				t.Fatal(err)
			}
			if back != tc.in {
				t.Errorf("ParseCursor(%q) = %+v, want %+v", tc.want, back, tc.in)
			}
		})
	}
}

// An empty cursor is the beginning, not a malformed one — the first call every
// consumer makes.
func TestParseCursorEmpty(t *testing.T) {
	got, err := ParseCursor("")
	if err != nil {
		t.Fatalf("empty cursor must parse: %v", err)
	}
	if got != (Cursor{}) {
		t.Errorf("cursor = %+v, want zero", got)
	}
}

func TestParseCursorRejectsGarbage(t *testing.T) {
	for _, s := range []string{"abc", "12", "abc:def", ":"} {
		if _, err := ParseCursor(s); err == nil {
			t.Errorf("ParseCursor(%q) accepted a malformed cursor", s)
		}
	}
}

func TestClientTorrents(t *testing.T) {
	path := newDB(t,
		`INSERT INTO client_searchee (client_host, info_hash, name, files, length)
		 VALUES ('h', 'aaa', 'Release.A',
		         '[{"name":"a.mkv","length":100,"path":"Release.A/a.mkv"}]', 100)`,
		// Same name, different hash: they are cross-seeds of each other. The
		// mapping is genuinely one-to-many and must not be collapsed here.
		`INSERT INTO client_searchee (client_host, info_hash, name, files, length)
		 VALUES ('h', 'bbb', 'Release.A',
		         '[{"name":"a.mkv","length":100,"path":"Release.A/a.mkv"}]', 100)`,
		// No name. Returned anyway: the infohash identifies it, and a row is
		// dropped only when it has no identifier at all. An earlier version
		// filtered these out because the consumer keyed on names.
		`INSERT INTO client_searchee (client_host, info_hash, name) VALUES ('h', 'ccc', NULL)`,
	)

	got, err := New(path).ClientTorrents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d torrents, want all 3 — the hash identifies them", len(got))
	}

	byHash := make(map[string]int)
	for _, c := range got {
		byHash[c.InfoHash] = len(c.Files)
	}
	// The file list is what separates two torrents sharing a name, so it has
	// to survive the JSON column rather than being dropped on the floor.
	if byHash["aaa"] != 1 || byHash["bbb"] != 1 {
		t.Errorf("file lists were lost: %v", byHash)
	}
	if _, ok := byHash["ccc"]; !ok {
		t.Error("a nameless searchee was dropped despite having a hash")
	}
}

// A files column that will not decode must not lose the row. The infohash,
// name and length are all still true — the consumer falls back to a weaker
// match for that one torrent rather than losing it entirely.
func TestClientTorrentsSurvivesBadFilesJSON(t *testing.T) {
	path := newDB(t,
		`INSERT INTO client_searchee (client_host, info_hash, name, files)
		 VALUES ('h', 'aaa', 'Release.A', 'not json')`,
	)

	got, err := New(path).ClientTorrents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d torrents, want the row kept", len(got))
	}
	if got[0].InfoHash != "aaa" || got[0].Files != nil {
		t.Errorf("row = %+v, want the hash kept and files empty", got[0])
	}
}

// cross-seed's own record of which infohash produced which linked path — the
// half that makes the graph hash-to-hash when an injected torrent lives in its
// own linkDir and shares no path with its source.
func TestEnsemble(t *testing.T) {
	path := newDB(t,
		`INSERT INTO ensemble (client_host, path, info_hash, ensemble, element)
		 VALUES ('h', '/mnt/media/cross-seed/links/X', 'aaa', 'Show.S01', 'E01')`,
	)

	got, err := New(path).Ensemble(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	if got[0].InfoHash != "aaa" || got[0].Element != "E01" {
		t.Errorf("entry = %+v", got[0])
	}
}

func TestStats(t *testing.T) {
	path := newDB(t,
		seedDecisions(3),
		`INSERT INTO client_searchee (client_host, info_hash, name) VALUES ('h', 'aaa', 'A')`,
	)

	got, err := New(path).Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Decisions != 3 || got.Searchees != 3 || got.ClientTorrents != 1 {
		t.Errorf("stats = %+v", got)
	}
}

func TestStatsEmptyDatabase(t *testing.T) {
	got, err := New(newDB(t)).Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Decisions != 0 || got.Searchees != 0 || got.ClientTorrents != 0 {
		t.Errorf("stats = %+v, want zeroes", got)
	}
}

func TestMissingDatabase(t *testing.T) {
	st := New(filepath.Join(t.TempDir(), "absent.db"))
	ctx := context.Background()

	if _, err := st.Stats(ctx); err == nil {
		t.Error("Stats: expected an error")
	}
	if _, err := st.Decisions(ctx, Cursor{}, 10); err == nil {
		t.Error("Decisions: expected an error")
	}
	if _, err := st.ClientTorrents(ctx); err == nil {
		t.Error("ClientTorrents: expected an error")
	}
}

// A file that exists but is not a database fails at query time rather than at
// open, because SQLite does not read the header until it has to.
func TestNotADatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cross-seed.db")
	if err := os.WriteFile(path, []byte("not sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := New(path).Stats(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
}

// The whole reason a snapshot is taken: cross-seed holds the live file open in
// WAL mode, and everything committed since the last checkpoint is in the -wal
// rather than the base file. Copying the base alone returns a stale view that
// looks perfectly valid — no error, just missing decisions.
func TestSnapshotIncludesWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cross-seed.db")

	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(seedDecisions(2)); err != nil {
		t.Fatal(err)
	}

	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Skipf("driver did not enter WAL mode (got %q), nothing to prove", mode)
	}
	if _, err := os.Stat(path + "-wal"); err != nil {
		t.Skipf("no -wal file on disk (%v), nothing to prove", err)
	}

	// Read while the writer still holds the file open, exactly as the sidecar
	// does against a running cross-seed.
	got, err := New(path).Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Decisions != 2 {
		t.Errorf("decisions = %d, want 2 — uncheckpointed rows were missed", got.Decisions)
	}
}

// One temp copy per request leaks a database-sized file per request otherwise.
func TestSnapshotCleanedUp(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())

	if _, err := New(newDB(t, seedDecisions(1))).Stats(context.Background()); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if len(e.Name()) >= 10 && e.Name()[:10] == "crossseed-" {
			t.Errorf("snapshot directory %q outlived the request", e.Name())
		}
	}
}

// Reading must never write. The sidecar runs as cross-seed's own user with
// access to its live database, so a read path that touches the file is the one
// failure that could damage the thing it reports on.
func TestReadDoesNotModifyTheDatabase(t *testing.T) {
	path := newDB(t, seedDecisions(3))

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	st := New(path)
	if _, err := st.Stats(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Decisions(ctx, Cursor{}, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClientTorrents(ctx); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("the database changed across three reads")
	}
}
