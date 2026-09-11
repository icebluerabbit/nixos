// Package store is stashtrap's SQLite persistence: the announce corpus and the
// decision taken on each one.
//
// Dumb persistence — CRUD and a couple of aggregates, no policy. SQLite rather
// than the homelab's PostgreSQL because there is exactly one writer (autobrr,
// one announce at a time) and no query a container-local file cannot serve.
// It lives on local disk: SQLite over NFS is a locking hazard.
package store

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"sort"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver, registered as "sqlite" (cgo-free)

	"homelab/stashtrap/internal/torrent"
	"homelab/stashtrap/internal/types"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store wraps the database.
type Store struct {
	db *sql.DB
}

// Open opens the database at path in WAL mode and applies pending migrations.
//
// SQLite is single-writer: every access is serialised through one warm
// connection so concurrent requests cannot produce SQLITE_BUSY. At this traffic
// — a handful of announces a minute — losing WAL's read concurrency costs
// nothing.
func Open(path string) (*Store, error) {
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxIdleTime(0)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close() // the open already failed; this is best-effort cleanup
		return nil, err
	}
	return s, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

// migrate applies every embedded migration not yet recorded, in name order.
func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS migrations (
		name TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	) STRICT`); err != nil {
		return fmt.Errorf("store: migrations table: %w", err)
	}

	names, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("store: list migrations: %w", err)
	}
	sort.Strings(names)

	for _, name := range names {
		var seen string
		switch err := s.db.QueryRow(
			`SELECT name FROM migrations WHERE name = ?`, name,
		).Scan(&seen); {
		case err == nil:
			continue
		case err != sql.ErrNoRows:
			return fmt.Errorf("store: check migration %s: %w", name, err)
		}

		body, err := migrationsFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("store: read migration %s: %w", name, err)
		}
		if _, err := s.db.Exec(string(body)); err != nil {
			return fmt.Errorf("store: apply migration %s: %w", name, err)
		}
		if _, err := s.db.Exec(
			`INSERT INTO migrations (name, applied_at) VALUES (?, ?)`,
			name, time.Now().UTC().Format(time.RFC3339Nano),
		); err != nil {
			return fmt.Errorf("store: record migration %s: %w", name, err)
		}
	}
	return nil
}

// Record is one announce plus what was decided about it.
type Record struct {
	// ReceivedAt is stashtrap's clock, not autobrr's: ordering our own
	// records should not depend on another service's time.
	ReceivedAt time.Time
	// Announce is the release as it arrived.
	Announce types.Announce
	// Torrent is the parsed .torrent, nil when none was sent.
	Torrent *torrent.Meta
	// TorrentError explains a torrent that arrived but would not parse.
	TorrentError string
	// Accepted is the decision.
	Accepted bool
	// Reason is why, in both directions.
	Reason string
	// Reading is what the catalogue recognised in the name. Stored beside
	// the announce rather than logged, so what a grab was for stays
	// answerable after the fact.
	Reading Reading
}

// Reading is the matching outcome for one announce.
type Reading struct {
	// SceneID is the scene the release was identified as, if any.
	SceneID string
	// Score is the evidence total behind the reading.
	Score float64
	// Corroborated reports that the parts of the reading agreed with one
	// another rather than each standing alone.
	Corroborated bool
	// Matches are the entities recognised, with the text that found them.
	Matches []RecordedMatch
}

// RecordedMatch is one entity a release was found to name.
type RecordedMatch struct {
	// Kind is performer, studio or scene.
	Kind string
	// EntityID is the stash-box id.
	EntityID string
	// Probe is the text that matched, in the release's own spelling.
	Probe string
	// Favourite reports whether this entity is followed, and so whether it
	// drove the decision.
	Favourite bool
}

// Insert writes one record and returns its id.
func (s *Store) Insert(ctx context.Context, r Record) (int64, error) {
	// A nil slice marshals to `null`, not `[]`, and the column's DEFAULT '[]'
	// never applies because the INSERT names it. json_each() over 'null' does
	// not behave like an empty array, so a rule counting tags would silently
	// skip every tag-less announce instead of seeing zero tags.
	announceTags := r.Announce.Tags
	if announceTags == nil {
		announceTags = []string{}
	}
	tags, err := json.Marshal(announceTags)
	if err != nil {
		return 0, fmt.Errorf("store: encode tags: %w", err)
	}

	// The infohash autobrr reports is preferred, because that is the value
	// qBittorrent will echo back; the computed one is the fallback for a
	// template that does not send it.
	infoHash := r.Announce.InfoHash
	files := []byte("[]")
	var (
		torrentName string
		fileCount   int
		totalSize   int64
		private     bool
		source      string
	)
	if r.Torrent != nil {
		if infoHash == "" {
			infoHash = r.Torrent.InfoHash
		}
		if files, err = json.Marshal(r.Torrent.Files); err != nil {
			return 0, fmt.Errorf("store: encode files: %w", err)
		}
		torrentName = r.Torrent.Name
		fileCount = len(r.Torrent.Files)
		totalSize = r.Torrent.TotalSize
		private = r.Torrent.Private
		source = r.Torrent.Source
	}

	// A transaction because an announce and its categories are one record:
	// a row with its categories missing would be worse than no row, since
	// nothing downstream could tell it apart from a genuinely uncategorised
	// release. The pool is capped at one connection, so this costs nothing.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("store: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once Commit has succeeded

	res, err := tx.ExecContext(ctx, `
		INSERT INTO announces (
			received_at, name, indexer, info_hash, torrent_id, category,
			tags, release_tags, size, freeleech, freeleech_percent,
			uploader, info_url, torrent_name, files, file_count,
			total_size, private, source, torrent_error, accepted, reason,
			scene_id, score, corroborated
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ReceivedAt.UTC().Format(time.RFC3339Nano),
		r.Announce.Name, r.Announce.Indexer, infoHash, r.Announce.TorrentID,
		r.Announce.Category, string(tags), r.Announce.ReleaseTags,
		announcedSize(r.Announce.Size), r.Announce.Freeleech, r.Announce.FreeleechPercent,
		r.Announce.Uploader, r.Announce.InfoURL,
		torrentName, string(files), fileCount, totalSize, private, source,
		r.TorrentError, r.Accepted, r.Reason,
		r.Reading.SceneID, r.Reading.Score, r.Reading.Corroborated,
	)
	if err != nil {
		return 0, fmt.Errorf("store: insert announce: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("store: insert id: %w", err)
	}

	if err := insertCategories(ctx, tx, id, types.ParseCategories(r.Announce.Categories)); err != nil {
		return 0, err
	}
	if err := insertMatches(ctx, tx, id, r.Reading.Matches); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("store: commit: %w", err)
	}
	return id, nil
}

// insertMatches records what the release was found to name.
//
// Same transaction as the announce: a row whose matches went missing would
// read as a release that matched nothing, which is the one thing it must never
// be confused with.
func insertMatches(ctx context.Context, tx *sql.Tx, announceID int64, matches []RecordedMatch) error {
	for _, m := range matches {
		if m.EntityID == "" {
			continue
		}
		// The same entity can be found twice in one name — once in the
		// header and once in the body — and that is one fact about the
		// release, not two.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO announce_matches (announce_id, kind, entity_id, probe, favourite)
			VALUES (?,?,?,?,?)
			ON CONFLICT (announce_id, kind, entity_id) DO UPDATE SET
				probe = excluded.probe, favourite = excluded.favourite`,
			announceID, m.Kind, m.EntityID, m.Probe, m.Favourite,
		); err != nil {
			return fmt.Errorf("store: record match %s %s: %w", m.Kind, m.EntityID, err)
		}
	}
	return nil
}

// insertCategories records the announce's categories and links them to it.
//
// The name is upserted rather than inserted once: an indexer that corrects a
// category name should correct it here too, and because categories are stored
// once rather than per announce, that is a single row. Prowlarr's BitPorn caps
// currently misspell every site-specific name with a trailing double quote, so
// this is the path that will repair them.
func insertCategories(ctx context.Context, tx *sql.Tx, announceID int64, cats []types.Category) error {
	for _, c := range cats {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO categories (id, name) VALUES (?, ?)
			ON CONFLICT (id) DO UPDATE SET name = excluded.name`,
			c.ID, c.Name,
		); err != nil {
			return fmt.Errorf("store: upsert category %d: %w", c.ID, err)
		}

		// A tracker repeating a category on one release is not an error,
		// and the link carries no payload, so the duplicate is dropped.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO announce_categories (announce_id, category_id) VALUES (?, ?)
			ON CONFLICT DO NOTHING`,
			announceID, c.ID,
		); err != nil {
			return fmt.Errorf("store: link category %d: %w", c.ID, err)
		}
	}
	return nil
}

// announcedSize narrows the announced size for SQLite, whose INTEGER is
// signed. A tracker reporting a size above 8 exabytes is reporting a bug, and
// clamping keeps it from being stored as a negative number.
func announcedSize(n uint64) int64 {
	if n > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(n)
}

// Count returns how many announces are recorded. Phase one has no reader
// beyond metrics and the odd sqlite3 session, and this is what the readiness
// check uses to prove the schema is usable.
func (s *Store) Count(ctx context.Context) (int64, error) {
	var n int64
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM announces`).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: count: %w", err)
	}
	return n, nil
}
