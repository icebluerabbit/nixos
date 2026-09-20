// Package store reads cross-seed's SQLite database.
//
// cross-seed has an HTTP API — /api/ping, /api/announce, /api/webhook — and
// none of it exposes decisions. Its webhook fires only when a candidate is
// INJECTED, so the API and the webhook together show the matches and none of
// the rejections, which are four fifths of the record and the only answer to
// "why was this not cross-seeded".
//
// This package is why the sidecar exists. Reading the file is the only way to
// get at that, and doing it from another container needed a bind mount, a
// matching uid and knowledge of a schema upstream is free to change. Running
// here instead — beside the database, inside the container that owns it —
// keeps all three local and turns the awkward part into an HTTP endpoint.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"homelab/clients/crossseed"

	// Pure-Go SQLite. No cgo, which keeps the Nix build free of a C toolchain
	// and the binary statically linked like every other one here.
	_ "modernc.org/sqlite"
)

// Cursor marks a position in the decision table.
//
// (first_seen, id), not first_seen alone. cross-seed writes several decisions
// per millisecond routinely — one search produces a verdict per candidate — so
// a timestamp cursor either re-reads a tied group forever or steps past part of
// it. The row id breaks every tie.
//
// Opaque to consumers: they store the encoded form and hand it back.
type Cursor struct {
	FirstSeenMS int64
	ID          int64
}

// String encodes a cursor for the wire.
func (c Cursor) String() string {
	if c == (Cursor{}) {
		return ""
	}
	return fmt.Sprintf("%d:%d", c.FirstSeenMS, c.ID)
}

// ParseCursor reads one back. An empty string is the beginning, not an error.
func ParseCursor(s string) (Cursor, error) {
	if s == "" {
		return Cursor{}, nil
	}
	var c Cursor
	if _, err := fmt.Sscanf(s, "%d:%d", &c.FirstSeenMS, &c.ID); err != nil {
		return Cursor{}, fmt.Errorf("store: bad cursor %q: %w", s, err)
	}
	return c, nil
}

// DecisionRow is one decision plus the cursor that resumes after it.
type DecisionRow struct {
	crossseed.Decision
	Cursor Cursor
}

// Store opens snapshots of the database on demand.
//
// A handle is NOT held open across requests. cross-seed writes to this file
// continuously, so a long-lived reader would serve whatever the database looked
// like when the sidecar started. Each request snapshots afresh, which costs a
// file copy and is the only way an answer is current.
type Store struct {
	path string
}

// New builds a store over the live database path.
func New(path string) *Store { return &Store{path: path} }

// snapshot copies the database and its WAL to a temporary location.
//
// Copying both matters: the WAL holds everything committed since the last
// checkpoint, and reading the base file alone silently returns a stale view.
// The copy is also what makes reading safe at all — cross-seed holds the live
// file open in WAL mode, and a reader must not be the thing that discovers what
// a half-applied WAL looks like.
func (s *Store) snapshot() (path string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "crossseed-*")
	if err != nil {
		return "", nil, fmt.Errorf("store: temp dir: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(dir) }

	dst := filepath.Join(dir, "cross-seed.db")
	// The base file, then the WAL. Not the -shm: it is shared-memory state
	// describing the WAL as of the moment it was written, and a copy paired
	// with a WAL that moved on is worse than none — SQLite rebuilds it from
	// the WAL itself.
	// gosec reads s.path as tainted. It is a --db-path flag set by the nix
	// module to a fixed store path, and nothing reachable from the HTTP surface
	// can influence it — this sidecar serves no path from a request.
	for _, suffix := range []string{"", "-wal"} {
		data, err := os.ReadFile(s.path + suffix) //nolint:gosec // operator-set flag, never request-derived
		if err != nil {
			if suffix == "" {
				cleanup()
				return "", nil, fmt.Errorf("store: read %s: %w", s.path, err)
			}
			// A database checkpointed and closed cleanly has no WAL.
			continue
		}
		// dst is under a MkdirTemp directory this function just created, and
		// suffix is one of two literals above. Tainted only by way of s.path.
		if err := os.WriteFile(dst+suffix, data, 0o600); err != nil { //nolint:gosec // destination is our own temp dir
			cleanup()
			return "", nil, fmt.Errorf("store: write snapshot: %w", err)
		}
	}
	return dst, cleanup, nil
}

// open returns a handle on a fresh snapshot.
//
// Opened writable, which looks wrong for a reader and is not: SQLite has to
// replay the copied WAL into the copied base file before it can answer
// anything, and that replay is a write. It lands on this process's private
// temp copy; cross-seed's own file is never opened at all.
//
// The two obvious-looking alternatives are both wrong here. mode=ro fails on a
// database with an unreplayed WAL (SQLITE_READONLY_RECOVERY), and immutable=1
// succeeds while silently skipping the WAL entirely — which answers from the
// base file as of the last checkpoint and looks like a perfectly valid result.
func (s *Store) open() (*sql.DB, func(), error) {
	snap, cleanup, err := s.snapshot()
	if err != nil {
		return nil, nil, err
	}
	db, err := sql.Open("sqlite", "file:"+snap)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("store: open %s: %w", snap, err)
	}
	return db, func() { _ = db.Close(); cleanup() }, nil
}

// Decisions returns a page of decisions after cursor, oldest first.
//
// Keyset paging, not OFFSET. cross-seed appends to this table continuously, so
// a walk that pages by offset shifts underneath itself: rows inserted mid-walk
// push later ones past a boundary already passed, and they are never read. A
// keyset cursor cannot skip, and it also stops the query re-scanning everything
// before the offset on every page.
func (s *Store) Decisions(ctx context.Context, after Cursor, limit int) ([]DecisionRow, error) {
	db, closeDB, err := s.open()
	if err != nil {
		return nil, err
	}
	defer closeDB()

	rows, err := db.QueryContext(ctx, `
		SELECT d.id, s.name, COALESCE(d.info_hash, ''), COALESCE(d.guid, ''),
		       COALESCE(d.decision, ''), d.first_seen,
		       COALESCE(d.last_seen, d.first_seen)
		  FROM decision d
		  JOIN searchee s ON s.id = d.searchee_id
		 WHERE (d.first_seen, d.id) > (?, ?)
		 ORDER BY d.first_seen, d.id
		 LIMIT ?`, after.FirstSeenMS, after.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: query decisions: %w", err)
	}
	// The close error on a read is not actionable: every row is already
	// decoded, and rows.Err below reports anything that went wrong reading them.
	defer func() { _ = rows.Close() }()

	out := make([]DecisionRow, 0, limit)
	for rows.Next() {
		var (
			r               DecisionRow
			id              int64
			firstMS, lastMS int64
		)
		if err := rows.Scan(&id, &r.SearcheeName, &r.InfoHash, &r.GUID,
			&r.Decision.Decision, &firstMS, &lastMS); err != nil {
			return nil, fmt.Errorf("store: scan decision: %w", err)
		}
		// cross-seed stores epoch milliseconds.
		r.FirstSeen = time.UnixMilli(firstMS).UTC()
		r.LastSeen = time.UnixMilli(lastMS).UTC()
		r.Cursor = Cursor{FirstSeenMS: firstMS, ID: id}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ClientTorrents returns what cross-seed knows the torrent client holds.
//
// The file list is the point. cross-seed keys a searchee by NAME and one name
// covers every cross-seed of that content, so a consumer given only names
// cannot say which infohash a decision was about. The files identify exactly
// one torrent, which is what lets the whole graph be expressed as infohash to
// infohash without matching a single title.
//
// An earlier version of this read name and info_hash alone and discarded the
// rest of the row. That left managarr resolving searchees by name, refusing
// every ambiguous one, and recording a source infohash for barely a quarter of
// cross-seed candidates.
func (s *Store) ClientTorrents(ctx context.Context) ([]crossseed.ClientTorrent, error) {
	db, closeDB, err := s.open()
	if err != nil {
		return nil, err
	}
	defer closeDB()

	rows, err := db.QueryContext(ctx, `
		SELECT info_hash,
		       COALESCE(name, ''),
		       COALESCE(title, ''),
		       COALESCE(files, ''),
		       COALESCE(length, 0),
		       COALESCE(save_path, ''),
		       COALESCE(category, '')
		  FROM client_searchee
		 WHERE info_hash IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("store: query client_searchee: %w", err)
	}
	// The close error on a read is not actionable: every row is already
	// decoded, and rows.Err below reports anything that went wrong reading them.
	defer func() { _ = rows.Close() }()

	var out []crossseed.ClientTorrent
	for rows.Next() {
		var (
			c        crossseed.ClientTorrent
			rawFiles string
		)
		if err := rows.Scan(&c.InfoHash, &c.Name, &c.Title, &rawFiles,
			&c.Length, &c.SavePath, &c.Category); err != nil {
			return nil, fmt.Errorf("store: scan client_searchee: %w", err)
		}
		// files is a JSON column. A row whose list will not decode keeps its
		// identity — the infohash, name and length are all still true — so it
		// is returned without files rather than dropped; the consumer falls
		// back to the weaker match for that one torrent instead of losing it.
		if rawFiles != "" {
			if err := json.Unmarshal([]byte(rawFiles), &c.Files); err != nil {
				c.Files = nil
			}
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Ensemble returns cross-seed's record of which infohash produced which path.
//
// The other half of a hash-to-hash graph. An injected cross-seed is written
// into its own linkDir, so it shares no path with the torrent it was matched
// against — this table is where cross-seed states that the two are the same
// content, by identifier.
func (s *Store) Ensemble(ctx context.Context) ([]crossseed.EnsembleEntry, error) {
	db, closeDB, err := s.open()
	if err != nil {
		return nil, err
	}
	defer closeDB()

	rows, err := db.QueryContext(ctx, `
		SELECT COALESCE(info_hash, ''),
		       COALESCE(path, ''),
		       COALESCE(ensemble, ''),
		       COALESCE(element, '')
		  FROM ensemble`)
	if err != nil {
		return nil, fmt.Errorf("store: query ensemble: %w", err)
	}
	// The close error on a read is not actionable: every row is already
	// decoded, and rows.Err below reports anything that went wrong reading them.
	defer func() { _ = rows.Close() }()

	var out []crossseed.EnsembleEntry
	for rows.Next() {
		var e crossseed.EnsembleEntry
		if err := rows.Scan(&e.InfoHash, &e.Path, &e.Ensemble, &e.Element); err != nil {
			return nil, fmt.Errorf("store: scan ensemble: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Stats returns the row counts, which is also the cheapest health check that
// actually touches the database.
func (s *Store) Stats(ctx context.Context) (crossseed.Stats, error) {
	db, closeDB, err := s.open()
	if err != nil {
		return crossseed.Stats{}, err
	}
	defer closeDB()

	var out crossseed.Stats
	for _, q := range []struct {
		sql  string
		into *int
	}{
		{"SELECT count(*) FROM decision", &out.Decisions},
		{"SELECT count(*) FROM searchee", &out.Searchees},
		{"SELECT count(*) FROM client_searchee", &out.ClientTorrents},
	} {
		if err := db.QueryRowContext(ctx, q.sql).Scan(q.into); err != nil {
			return out, fmt.Errorf("store: %s: %w", q.sql, err)
		}
	}
	return out, nil
}
