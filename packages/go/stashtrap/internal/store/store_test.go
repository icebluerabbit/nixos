package store_test

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/go-cmp/cmp"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"homelab/stashtrap/internal/store"
	"homelab/stashtrap/internal/torrent"
	"homelab/stashtrap/internal/types"
)

func open(t *testing.T) (st *store.Store, path string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "stashtrap.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st, path
}

func TestInsertAndCount(t *testing.T) {
	t.Parallel()

	st, path := open(t)
	ctx := context.Background()

	if n, err := st.Count(ctx); err != nil || n != 0 {
		t.Fatalf("Count on empty = %d, %v", n, err)
	}

	id, err := st.Insert(ctx, store.Record{
		ReceivedAt: time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC),
		Announce: types.Announce{
			Name:        "[Site.com] Performer - Title",
			Indexer:     "bitporn",
			InfoHash:    "reported-hash",
			Tags:        []string{"anal", "teen"},
			Size:        4096,
			Freeleech:   true,
			DownloadURL: "https://tracker.invalid/download?passkey=SECRET",
		},
		Torrent: &torrent.Meta{
			InfoHash:  "computed-hash",
			Name:      "Actress Pack",
			Files:     []torrent.File{{Path: "a.mp4", Length: 1}, {Path: "b.mp4", Length: 2}},
			TotalSize: 3,
			Private:   true,
			Source:    "BitPorn",
		},
		Accepted: false,
		Reason:   "record-only",
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if id != 1 {
		t.Fatalf("id = %d, want 1", id)
	}
	if n, err := st.Count(ctx); err != nil || n != 1 {
		t.Fatalf("Count = %d, %v", n, err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()

	var (
		infoHash, tags, files, source string
		fileCount, totalSize          int64
		private, accepted             bool
	)
	if err := db.QueryRow(`
		SELECT info_hash, tags, files, file_count, total_size, private, source, accepted
		FROM announces WHERE id = ?`, id,
	).Scan(&infoHash, &tags, &files, &fileCount, &totalSize, &private, &source, &accepted); err != nil {
		t.Fatalf("select: %v", err)
	}

	// autobrr's hash wins: it is the value qBittorrent will echo back.
	if infoHash != "reported-hash" {
		t.Fatalf("info_hash = %q, want the reported one", infoHash)
	}
	var gotTags []string
	if err := json.Unmarshal([]byte(tags), &gotTags); err != nil || len(gotTags) != 2 {
		t.Fatalf("tags = %q (%v)", tags, err)
	}
	var gotFiles []torrent.File
	if err := json.Unmarshal([]byte(files), &gotFiles); err != nil || len(gotFiles) != 2 {
		t.Fatalf("files = %q (%v)", files, err)
	}
	if fileCount != 2 || totalSize != 3 || !private || source != "BitPorn" || accepted {
		t.Fatalf("derived columns wrong: %d %d %v %q %v", fileCount, totalSize, private, source, accepted)
	}

	// The passkey-bearing download URL has no column, so it cannot be
	// stored by accident.
	var count int
	if err := db.QueryRow(
		`SELECT count(*) FROM pragma_table_info('announces') WHERE name = 'download_url'`,
	).Scan(&count); err != nil {
		t.Fatalf("pragma: %v", err)
	}
	if count != 0 {
		t.Fatal("schema has a download_url column; the passkey would be persisted")
	}
}

// Without a torrent the derived columns keep their defaults, and the reported
// infohash still lands.
func TestInsertWithoutTorrent(t *testing.T) {
	t.Parallel()

	st, _ := open(t)
	if _, err := st.Insert(context.Background(), store.Record{
		ReceivedAt:   time.Now(),
		Announce:     types.Announce{Name: "x", Indexer: "bitporn"},
		TorrentError: "torrent: no info dictionary",
		Reason:       "record-only",
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
}

// tags is queried with json1, and json_each() over the string 'null' does not
// behave like an empty array — a rule counting tags would skip every tag-less
// announce rather than see zero. A nil slice marshals to `null`, and the
// column's DEFAULT '[]' cannot help because the INSERT names the column.
func TestInsertStoresEmptyTagsAsAnArray(t *testing.T) {
	t.Parallel()

	tests := map[string][]string{
		"nil":   nil,
		"empty": {},
		"set":   {"a", "b"},
	}

	for name, tags := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			st, path := open(t)
			id, err := st.Insert(context.Background(), store.Record{
				ReceivedAt: time.Now(),
				Announce:   types.Announce{Name: "x", Indexer: "bitporn", Tags: tags},
				Reason:     "record-only",
			})
			if err != nil {
				t.Fatalf("Insert: %v", err)
			}

			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer db.Close()

			var stored string
			if err := db.QueryRow(`SELECT tags FROM announces WHERE id = ?`, id).Scan(&stored); err != nil {
				t.Fatalf("select tags: %v", err)
			}

			want, err := json.Marshal(append([]string{}, tags...))
			if err != nil {
				t.Fatalf("marshal want: %v", err)
			}
			if stored != string(want) {
				t.Errorf("tags = %q, want %q", stored, want)
			}

			// The property that actually matters: json1 must be able to
			// walk it, yielding one row per tag and none for empty.
			var counted int
			if err := db.QueryRow(
				`SELECT count(*) FROM announces, json_each(announces.tags) WHERE announces.id = ?`, id,
			).Scan(&counted); err != nil {
				t.Fatalf("json_each: %v", err)
			}
			if counted != len(tags) {
				t.Errorf("json_each yielded %d rows, want %d", counted, len(tags))
			}
		})
	}
}

// Reopening applies no migration twice and keeps the rows.
func TestOpenIsIdempotent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "stashtrap.db")
	first, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	ctx := context.Background()
	if _, err := first.Insert(ctx, store.Record{
		ReceivedAt: time.Now(),
		Announce:   types.Announce{Name: "x", Indexer: "bitporn"},
		Reason:     "record-only",
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	second, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer second.Close()
	if n, err := second.Count(ctx); err != nil || n != 1 {
		t.Fatalf("Count after reopen = %d, %v", n, err)
	}
}

func TestOpenRejectsUnusablePath(t *testing.T) {
	t.Parallel()

	if _, err := store.Open(filepath.Join(t.TempDir(), "missing-dir", "x.db")); err == nil {
		t.Fatal("Open succeeded with a nonexistent directory")
	}
}

// categoriesOf reads back an announce's categories through the join.
func categoriesOf(t *testing.T, path string, id int64) map[int]string {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT c.id, c.name FROM categories c
		JOIN announce_categories ac ON ac.category_id = c.id
		WHERE ac.announce_id = ?`, id)
	if err != nil {
		t.Fatalf("query categories: %v", err)
	}
	defer rows.Close()

	out := map[int]string{}
	for rows.Next() {
		var cid int
		var name string
		if err := rows.Scan(&cid, &name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[cid] = name
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return out
}

func TestInsertRecordsCategories(t *testing.T) {
	t.Parallel()

	st, path := open(t)
	id, err := st.Insert(context.Background(), store.Record{
		ReceivedAt: time.Now(),
		Announce: types.Announce{
			Name: "x", Indexer: "torznab-bitporn",
			Categories: []string{"XXX", "6000", "Fansite", "100020"},
		},
		Reason: "record-only",
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	want := map[int]string{6000: "XXX", 100020: "Fansite"}
	if diff := cmp.Diff(want, categoriesOf(t, path, id)); diff != "" {
		t.Errorf("categories mismatch (-want +got):\n%s", diff)
	}
}

// The whole point of a join table: a category is stored once however many
// announces reference it, so repairing an upstream name is one row.
func TestCategoriesAreSharedAndNamesUpsert(t *testing.T) {
	t.Parallel()

	st, path := open(t)
	ctx := context.Background()

	for _, name := range []string{`Fansite"`, `Fansite"`, "Fansite"} {
		if _, err := st.Insert(ctx, store.Record{
			ReceivedAt: time.Now(),
			Announce: types.Announce{
				Name: "x", Indexer: "torznab-bitporn",
				Categories: []string{"XXX", "6000", name, "100020"},
			},
			Reason: "record-only",
		}); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	var rows int
	if err := db.QueryRow(`SELECT count(*) FROM categories`).Scan(&rows); err != nil {
		t.Fatalf("count categories: %v", err)
	}
	if rows != 2 {
		t.Errorf("categories rows = %d, want 2 for three announces", rows)
	}

	// The corrected name wins, so fixing it upstream fixes it here.
	var name string
	if err := db.QueryRow(`SELECT name FROM categories WHERE id = 100020`).Scan(&name); err != nil {
		t.Fatalf("select name: %v", err)
	}
	if name != "Fansite" {
		t.Errorf("name = %q, want the latest announce's %q", name, "Fansite")
	}

	var links int
	if err := db.QueryRow(`SELECT count(*) FROM announce_categories`).Scan(&links); err != nil {
		t.Fatalf("count links: %v", err)
	}
	if links != 6 {
		t.Errorf("links = %d, want 6 (three announces x two categories)", links)
	}
}

// An announce with no categories is valid — an IRC release has none — and must
// not fail or leave a partial row.
func TestInsertWithoutCategories(t *testing.T) {
	t.Parallel()

	st, path := open(t)
	id, err := st.Insert(context.Background(), store.Record{
		ReceivedAt: time.Now(),
		Announce:   types.Announce{Name: "x", Indexer: "irc"},
		Reason:     "record-only",
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if got := categoriesOf(t, path, id); len(got) != 0 {
		t.Errorf("categories = %v, want none", got)
	}
}
