package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Normalise folds an alias into its lookup form: lower case, with everything
// that is not a letter or digit removed.
//
// Release names separate words with dots, underscores, spaces or nothing at
// all, so `jane.doe`, `Jane Doe` and `jane_doe` have to collide.
// Doing it at write time keeps the matcher's lookup a plain index hit.
func Normalise(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Performer is one catalogue performer as stashtrap stores it.
type Performer struct {
	ID             string
	Name           string
	Disambiguation string
	Gender         string
	Aliases        []string
	URLs           []CatalogueURL
	IsFavourite    bool
	Deleted        bool
	MergedIntoID   string
	UpdatedAt      string
}

// Studio is one catalogue studio.
type Studio struct {
	ID          string
	Name        string
	ParentID    string
	Aliases     []string
	URLs        []CatalogueURL
	IsFavourite bool
	Deleted     bool
	UpdatedAt   string
}

// Scene is one catalogue scene with its joins.
type Scene struct {
	ID           string
	Title        string
	ReleaseDate  string
	Code         string
	Duration     int
	StudioID     string
	Deleted      bool
	UpdatedAt    string
	Performers   []ScenePerformer
	Tags         []Tag
	URLs         []CatalogueURL
	Fingerprints []Fingerprint
}

// ScenePerformer is a performer's appearance in a scene, under the alias that
// scene credited them with.
type ScenePerformer struct {
	PerformerID string
	As          string
}

// Tag is a scene tag.
type Tag struct {
	ID   string
	Name string
}

// CatalogueURL is a link with the site hosting it.
type CatalogueURL struct {
	URL  string
	Site string
}

// Fingerprint is a hash submitted for a scene.
type Fingerprint struct {
	Algorithm   string
	Hash        string
	Duration    int
	Submissions int
}

// UpsertPerformer writes one performer and replaces its aliases and urls.
//
// Replace rather than merge: stash-box is the source of truth, and an alias
// removed there has to disappear here or the matcher keeps hitting on it.
func (s *Store) UpsertPerformer(ctx context.Context, endpoint string, p Performer) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO performers (
				endpoint, id, name, disambiguation, gender,
				is_favourite, deleted, merged_into_id, updated_at, synced_at
			) VALUES (?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT (endpoint, id) DO UPDATE SET
				name = excluded.name,
				disambiguation = excluded.disambiguation,
				gender = excluded.gender,
				is_favourite = excluded.is_favourite,
				deleted = excluded.deleted,
				merged_into_id = excluded.merged_into_id,
				updated_at = excluded.updated_at,
				synced_at = excluded.synced_at`,
			endpoint, p.ID, p.Name, p.Disambiguation, p.Gender,
			p.IsFavourite, p.Deleted, p.MergedIntoID, p.UpdatedAt, now,
		); err != nil {
			return fmt.Errorf("store: upsert performer %s: %w", p.ID, err)
		}

		if _, err := tx.ExecContext(ctx,
			`DELETE FROM performer_aliases WHERE endpoint = ? AND performer_id = ?`,
			endpoint, p.ID,
		); err != nil {
			return fmt.Errorf("store: clear aliases for %s: %w", p.ID, err)
		}
		// The canonical name is an alias too: a release naming the performer
		// by their primary name must hit the same index.
		for _, alias := range append([]string{p.Name}, p.Aliases...) {
			if alias == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO performer_aliases (endpoint, performer_id, alias, normalised)
				VALUES (?,?,?,?)`,
				endpoint, p.ID, alias, Normalise(alias),
			); err != nil {
				return fmt.Errorf("store: alias %q for %s: %w", alias, p.ID, err)
			}
		}

		if _, err := tx.ExecContext(ctx,
			`DELETE FROM performer_urls WHERE endpoint = ? AND performer_id = ?`,
			endpoint, p.ID,
		); err != nil {
			return fmt.Errorf("store: clear urls for %s: %w", p.ID, err)
		}
		for _, u := range p.URLs {
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO performer_urls (endpoint, performer_id, url, site)
				VALUES (?,?,?,?)`,
				endpoint, p.ID, u.URL, u.Site,
			); err != nil {
				return fmt.Errorf("store: url for %s: %w", p.ID, err)
			}
		}
		return nil
	})
}

// UpsertStudio writes one studio and replaces its aliases and urls.
func (s *Store) UpsertStudio(ctx context.Context, endpoint string, st Studio) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO studios (
				endpoint, id, name, parent_id, is_favourite, deleted, updated_at, synced_at
			) VALUES (?,?,?,?,?,?,?,?)
			ON CONFLICT (endpoint, id) DO UPDATE SET
				name = excluded.name,
				parent_id = excluded.parent_id,
				is_favourite = excluded.is_favourite,
				deleted = excluded.deleted,
				updated_at = excluded.updated_at,
				synced_at = excluded.synced_at`,
			endpoint, st.ID, st.Name, st.ParentID, st.IsFavourite, st.Deleted, st.UpdatedAt, now,
		); err != nil {
			return fmt.Errorf("store: upsert studio %s: %w", st.ID, err)
		}

		if _, err := tx.ExecContext(ctx,
			`DELETE FROM studio_aliases WHERE endpoint = ? AND studio_id = ?`,
			endpoint, st.ID,
		); err != nil {
			return fmt.Errorf("store: clear studio aliases for %s: %w", st.ID, err)
		}
		for _, alias := range append([]string{st.Name}, st.Aliases...) {
			if alias == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO studio_aliases (endpoint, studio_id, alias, normalised)
				VALUES (?,?,?,?)`,
				endpoint, st.ID, alias, Normalise(alias),
			); err != nil {
				return fmt.Errorf("store: studio alias %q for %s: %w", alias, st.ID, err)
			}
		}

		if _, err := tx.ExecContext(ctx,
			`DELETE FROM studio_urls WHERE endpoint = ? AND studio_id = ?`,
			endpoint, st.ID,
		); err != nil {
			return fmt.Errorf("store: clear studio urls for %s: %w", st.ID, err)
		}
		for _, u := range st.URLs {
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO studio_urls (endpoint, studio_id, url, site)
				VALUES (?,?,?,?)`,
				endpoint, st.ID, u.URL, u.Site,
			); err != nil {
				return fmt.Errorf("store: studio url for %s: %w", st.ID, err)
			}
		}
		return nil
	})
}

// UpsertScene writes one scene and replaces its performer, tag, url and
// fingerprint joins.
func (s *Store) UpsertScene(ctx context.Context, endpoint string, sc Scene) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO scenes (
				endpoint, id, title, title_normalised, release_date, code, duration,
				studio_id, deleted, updated_at, synced_at
			) VALUES (?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT (endpoint, id) DO UPDATE SET
				title = excluded.title,
				title_normalised = excluded.title_normalised,
				release_date = excluded.release_date,
				code = excluded.code,
				duration = excluded.duration,
				studio_id = excluded.studio_id,
				deleted = excluded.deleted,
				updated_at = excluded.updated_at,
				synced_at = excluded.synced_at`,
			endpoint, sc.ID, sc.Title, Normalise(sc.Title), sc.ReleaseDate, sc.Code, sc.Duration,
			sc.StudioID, sc.Deleted, sc.UpdatedAt, now,
		); err != nil {
			return fmt.Errorf("store: upsert scene %s: %w", sc.ID, err)
		}

		// The search index is maintained here rather than by a trigger:
		// FTS5 contentless tables cannot be updated in place, so the row is
		// replaced whenever the scene is.
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM scene_search WHERE endpoint = ? AND scene_id = ?`,
			endpoint, sc.ID,
		); err != nil {
			return fmt.Errorf("store: clear scene search row for %s: %w", sc.ID, err)
		}
		if sc.Title != "" && !sc.Deleted {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO scene_search (title, scene_id, endpoint) VALUES (?,?,?)`,
				sc.Title, sc.ID, endpoint,
			); err != nil {
				return fmt.Errorf("store: index scene title for %s: %w", sc.ID, err)
			}
		}

		for _, table := range []string{"scene_performers", "scene_tags", "scene_urls", "scene_fingerprints"} {
			if _, err := tx.ExecContext(ctx,
				//nolint:gosec // table comes from the fixed list above, never from input
				`DELETE FROM `+table+` WHERE endpoint = ? AND scene_id = ?`,
				endpoint, sc.ID,
			); err != nil {
				return fmt.Errorf("store: clear %s for %s: %w", table, sc.ID, err)
			}
		}

		for _, p := range sc.Performers {
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO scene_performers (endpoint, scene_id, performer_id, as_alias, normalised)
				VALUES (?,?,?,?,?)`,
				endpoint, sc.ID, p.PerformerID, p.As, Normalise(p.As),
			); err != nil {
				return fmt.Errorf("store: scene performer for %s: %w", sc.ID, err)
			}
		}
		for _, t := range sc.Tags {
			if _, err := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO tags (endpoint, id, name) VALUES (?,?,?)`,
				endpoint, t.ID, t.Name,
			); err != nil {
				return fmt.Errorf("store: tag %s: %w", t.ID, err)
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO scene_tags (endpoint, scene_id, tag_id) VALUES (?,?,?)`,
				endpoint, sc.ID, t.ID,
			); err != nil {
				return fmt.Errorf("store: scene tag for %s: %w", sc.ID, err)
			}
		}
		for _, u := range sc.URLs {
			if _, err := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO scene_urls (endpoint, scene_id, url, site) VALUES (?,?,?,?)`,
				endpoint, sc.ID, u.URL, u.Site,
			); err != nil {
				return fmt.Errorf("store: scene url for %s: %w", sc.ID, err)
			}
		}
		for _, f := range sc.Fingerprints {
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO scene_fingerprints (endpoint, scene_id, algorithm, hash, duration, submissions)
				VALUES (?,?,?,?,?,?)`,
				endpoint, sc.ID, f.Algorithm, f.Hash, f.Duration, f.Submissions,
			); err != nil {
				return fmt.Errorf("store: fingerprint for %s: %w", sc.ID, err)
			}
		}
		return nil
	})
}

// FavouritePerformerIDs lists the performers the account follows.
func (s *Store) FavouritePerformerIDs(ctx context.Context, endpoint string) ([]string, error) {
	return s.ids(ctx,
		`SELECT id FROM performers WHERE endpoint = ? AND is_favourite = 1 AND deleted = 0 ORDER BY id`,
		endpoint)
}

// FavouriteStudioIDs lists the studios the account follows.
func (s *Store) FavouriteStudioIDs(ctx context.Context, endpoint string) ([]string, error) {
	return s.ids(ctx,
		`SELECT id FROM studios WHERE endpoint = ? AND is_favourite = 1 AND deleted = 0 ORDER BY id`,
		endpoint)
}

// CatalogueCounts reports how much of the catalogue is stored, for the sync
// summary and the readiness of any matching built on it.
func (s *Store) CatalogueCounts(ctx context.Context, endpoint string) (performers, studios, scenes int64, err error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT count(*) FROM performers WHERE endpoint = ?),
			(SELECT count(*) FROM studios    WHERE endpoint = ?),
			(SELECT count(*) FROM scenes     WHERE endpoint = ?)`,
		endpoint, endpoint, endpoint)
	if err := row.Scan(&performers, &studios, &scenes); err != nil {
		return 0, 0, 0, fmt.Errorf("store: catalogue counts: %w", err)
	}
	return performers, studios, scenes, nil
}

// StartSync records that a crawl began, clearing the previous outcome.
func (s *Store) StartSync(ctx context.Context, endpoint, entity string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sync_state (endpoint, entity, started_at, finished_at, items, error)
		VALUES (?,?,?,'',0,'')
		ON CONFLICT (endpoint, entity) DO UPDATE SET
			started_at = excluded.started_at, finished_at = '', items = 0, error = ''`,
		endpoint, entity, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("store: start sync %s: %w", entity, err)
	}
	return nil
}

// FinishSync records the outcome of a crawl. A failed crawl keeps its error so
// a stale catalogue is visible rather than silent.
func (s *Store) FinishSync(ctx context.Context, endpoint, entity string, items int, syncErr error) error {
	msg := ""
	if syncErr != nil {
		msg = syncErr.Error()
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE sync_state SET finished_at = ?, items = ?, error = ?
		WHERE endpoint = ? AND entity = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), items, msg, endpoint, entity)
	if err != nil {
		return fmt.Errorf("store: finish sync %s: %w", entity, err)
	}
	return nil
}

// ids runs a single-column query.
func (s *Store) ids(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query ids: %w", err)
	}
	defer func() { _ = rows.Close() }() // the error that matters is rows.Err below

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: scan id: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate ids: %w", err)
	}
	return out, nil
}

// tx runs fn in a transaction. Every upsert touches several tables, and a
// half-written performer whose aliases were cleared but not rewritten would
// silently stop matching.
func (s *Store) tx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	return nil
}
