package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Kinds of catalogue entry a probe can hit.
const (
	KindScene     = "scene"
	KindPerformer = "performer"
	KindStudio    = "studio"
)

// Sources an alias came from, weakest last. Where a name was written matters:
// a studio's own name is a stronger claim than a domain derived from its
// website, and a performer's canonical name is stronger than an alias one
// scene happened to credit them under.
const (
	SourceName       = "name"
	SourceAlias      = "alias"
	SourceSceneTitle = "scene_title"
	SourceCredit     = "credit"
)

// Hit is one catalogue entry a probe matched.
type Hit struct {
	// Probe is the normalised text that matched.
	Probe string
	// Kind is scene, performer or studio.
	Kind string
	// ID is the stash-box id.
	ID string
	// Source says which table produced it.
	Source string
	// Shared is how many distinct entities of this kind answer to this
	// probe. One is an identification; forty is a first name.
	Shared int
}

// Lookup matches every probe against every kind of catalogue entry at once.
//
// One query rather than three per chunk: the probes for a release number in the
// dozens, and the alternative is a round trip per probe per table. Nothing here
// filters by what a probe "should" be — a header probe is asked about
// performers and a body probe about studios, because release names do not
// respect that distinction.
func (s *Store) Lookup(ctx context.Context, endpoint string, probes []string) ([]Hit, error) {
	probes = unique(probes)
	if len(probes) == 0 {
		return nil, nil
	}

	values := make([]string, 0, len(probes))
	args := make([]any, 0, len(probes)+4)
	for _, p := range probes {
		values = append(values, "(?)")
		args = append(args, p)
	}
	// One endpoint argument per SELECT below.
	args = append(args, endpoint, endpoint, endpoint, endpoint)

	// The share of each alias comes from alias_freq, computed once per crawl.
	// Left-joined rather than joined: an alias added since the last crawl is
	// worth scoring as unique rather than not at all.
	//
	//nolint:gosec // the only interpolation is a "(?),(?)" list built from a
	// count and package constants; every probe is a bound parameter.
	query := `
		WITH probe(text) AS (VALUES ` + strings.Join(values, ",") + `)
		SELECT p.text, '` + KindScene + `', s.id, '` + SourceSceneTitle + `', coalesce(f.entities, 1)
		  FROM probe p JOIN scenes s
		    ON s.title_normalised = p.text AND s.deleted = 0 AND s.endpoint = ?
		  LEFT JOIN alias_freq f
		    ON f.endpoint = s.endpoint AND f.kind = '` + KindScene + `' AND f.normalised = p.text
		UNION ALL
		SELECT DISTINCT p.text, '` + KindPerformer + `', pa.performer_id, '` + SourceAlias + `', coalesce(f.entities, 1)
		  FROM probe p JOIN performer_aliases pa
		    ON pa.normalised = p.text AND pa.endpoint = ?
		  LEFT JOIN alias_freq f
		    ON f.endpoint = pa.endpoint AND f.kind = '` + KindPerformer + `' AND f.normalised = p.text
		UNION ALL
		SELECT DISTINCT p.text, '` + KindPerformer + `', sp.performer_id, '` + SourceCredit + `', coalesce(f.entities, 1)
		  FROM probe p JOIN scene_performers sp
		    ON sp.normalised = p.text AND sp.endpoint = ?
		  LEFT JOIN alias_freq f
		    ON f.endpoint = sp.endpoint AND f.kind = '` + KindPerformer + `' AND f.normalised = p.text
		UNION ALL
		SELECT DISTINCT p.text, '` + KindStudio + `', sa.studio_id, '` + SourceAlias + `', coalesce(f.entities, 1)
		  FROM probe p JOIN studio_aliases sa
		    ON sa.normalised = p.text AND sa.endpoint = ?
		  LEFT JOIN alias_freq f
		    ON f.endpoint = sa.endpoint AND f.kind = '` + KindStudio + `' AND f.normalised = p.text`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: lookup probes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.Probe, &h.Kind, &h.ID, &h.Source, &h.Shared); err != nil {
			return nil, fmt.Errorf("store: scan hit: %w", err)
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate hits: %w", err)
	}
	return out, nil
}

// SceneFacts are the relations a scene has, for checking whether a reading's
// parts agree with each other.
type SceneFacts struct {
	ID          string
	Title       string
	StudioID    string
	ParentID    string
	ReleaseDate string
	Performers  []string
}

// Scenes returns the facts for these scene ids.
func (s *Store) Scenes(ctx context.Context, endpoint string, ids []string) ([]SceneFacts, error) {
	ids = unique(ids)
	if len(ids) == 0 {
		return nil, nil
	}

	args := []any{endpoint}
	for _, id := range ids {
		args = append(args, id)
	}
	//nolint:gosec // the interpolation is a bind-parameter list built from a
	// count; the ids themselves are bound.
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.title, s.studio_id, coalesce(st.parent_id, ''), s.release_date,
		       coalesce(group_concat(sp.performer_id), '')
		  FROM scenes s
		  LEFT JOIN studios st ON st.endpoint = s.endpoint AND st.id = s.studio_id
		  LEFT JOIN scene_performers sp ON sp.endpoint = s.endpoint AND sp.scene_id = s.id
		 WHERE s.endpoint = ? AND s.id IN (`+placeholders(len(ids))+`)
		 GROUP BY s.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: scene facts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []SceneFacts
	for rows.Next() {
		var f SceneFacts
		var performers string
		if err := rows.Scan(&f.ID, &f.Title, &f.StudioID, &f.ParentID, &f.ReleaseDate, &performers); err != nil {
			return nil, fmt.Errorf("store: scan scene facts: %w", err)
		}
		if performers != "" {
			f.Performers = strings.Split(performers, ",")
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate scene facts: %w", err)
	}
	return out, nil
}

// PendingFavourite is a recorded release that named a followed entity but was
// never acted on.
type PendingFavourite struct {
	ID       int64
	Name     string
	InfoHash string
	Reason   string
	// Entities are the followed ids the release named.
	Entities []string
}

// UngrabbedFavourites lists releases that named a followed entity and were not
// accepted.
//
// These exist because the decision a release got was the decision the service
// was making at the time: everything recorded before the accept rule existed
// was rejected regardless of who was in it. Nothing about them has changed
// except what is now known, and the .torrent is already stored, so they can be
// acted on without asking the tracker for anything.
func (s *Store) UngrabbedFavourites(ctx context.Context) ([]PendingFavourite, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.name, a.info_hash, a.reason,
		       group_concat(DISTINCT m.entity_id)
		  FROM announces a
		  JOIN announce_matches m ON m.announce_id = a.id
		 WHERE m.favourite = 1 AND a.accepted = 0
		 GROUP BY a.id
		 ORDER BY a.id`)
	if err != nil {
		return nil, fmt.Errorf("store: ungrabbed favourites: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []PendingFavourite
	for rows.Next() {
		var p PendingFavourite
		var entities string
		if err := rows.Scan(&p.ID, &p.Name, &p.InfoHash, &p.Reason, &entities); err != nil {
			return nil, fmt.Errorf("store: scan pending favourite: %w", err)
		}
		if entities != "" {
			p.Entities = strings.Split(entities, ",")
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate pending favourites: %w", err)
	}
	return out, nil
}

// Favourites narrows resolved ids to the ones that are followed.
//
// The decision turns on this and nothing else, so it is asked as its own
// question rather than inferred from a flag carried along with a match.
func (s *Store) Favourites(ctx context.Context, endpoint string, performerIDs, studioIDs []string) (performers, studios []string, err error) {
	performerIDs = unique(performerIDs)
	studioIDs = unique(studioIDs)

	if len(performerIDs) > 0 {
		args := []any{endpoint}
		for _, id := range performerIDs {
			args = append(args, id)
		}
		performers, err = s.ids(ctx, `
			SELECT id FROM performers
			 WHERE endpoint = ? AND is_favourite = 1 AND deleted = 0
			   AND id IN (`+placeholders(len(performerIDs))+`)`, args...)
		if err != nil {
			return nil, nil, fmt.Errorf("store: favourite performers: %w", err)
		}
	}

	if len(studioIDs) > 0 {
		args := []any{endpoint}
		for _, id := range studioIDs {
			args = append(args, id)
		}
		studios, err = s.ids(ctx, `
			SELECT id FROM studios
			 WHERE endpoint = ? AND is_favourite = 1 AND deleted = 0
			   AND id IN (`+placeholders(len(studioIDs))+`)`, args...)
		if err != nil {
			return nil, nil, fmt.Errorf("store: favourite studios: %w", err)
		}
	}
	return performers, studios, nil
}

// EntityNames maps ids to the names the catalogue holds for them.
//
// For logs and dashboards: an id identifies an entity to a database and to
// nobody else, and a panel showing which followed performers actually turn up
// is only worth having if it says who.
func (s *Store) EntityNames(ctx context.Context, endpoint string, performerIDs, studioIDs []string) (map[string]string, error) {
	out := map[string]string{}
	for _, q := range []struct {
		table string
		ids   []string
	}{{"performers", unique(performerIDs)}, {"studios", unique(studioIDs)}} {
		if len(q.ids) == 0 {
			continue
		}
		args := []any{endpoint}
		for _, id := range q.ids {
			args = append(args, id)
		}
		//nolint:gosec // the table comes from the fixed list above and the
		// interpolation is a bind-parameter list sized by a count.
		rows, err := s.db.QueryContext(ctx,
			`SELECT id, name FROM `+q.table+`
			  WHERE endpoint = ? AND id IN (`+placeholders(len(q.ids))+`)`, args...)
		if err != nil {
			return nil, fmt.Errorf("store: %s names: %w", q.table, err)
		}
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("store: scan %s name: %w", q.table, err)
			}
			out[id] = name
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, fmt.Errorf("store: iterate %s names: %w", q.table, err)
		}
	}
	return out, nil
}

// FavouriteAlias is a name one of the followed entities answers to.
type FavouriteAlias struct {
	Kind       string
	ID         string
	Name       string
	Alias      string
	Normalised string
}

// FavouriteAliases lists every name a followed performer or studio is known by.
//
// The decision this service exists to make turns on these and nothing else, so
// they are worth having as a set rather than as a query: it is what an accept
// rule checks against, and what a sweep for releases the matcher should have
// caught is measured against.
func (s *Store) FavouriteAliases(ctx context.Context, endpoint string) ([]FavouriteAlias, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT '`+KindPerformer+`', p.id, p.name, pa.alias, pa.normalised
		  FROM performers p JOIN performer_aliases pa
		    ON pa.endpoint = p.endpoint AND pa.performer_id = p.id
		 WHERE p.endpoint = ? AND p.is_favourite = 1 AND p.deleted = 0
		UNION
		SELECT '`+KindPerformer+`', p.id, p.name, sp.as_alias, sp.normalised
		  FROM performers p JOIN scene_performers sp
		    ON sp.endpoint = p.endpoint AND sp.performer_id = p.id
		 WHERE p.endpoint = ? AND p.is_favourite = 1 AND p.deleted = 0
		UNION
		SELECT '`+KindStudio+`', st.id, st.name, sa.alias, sa.normalised
		  FROM studios st JOIN studio_aliases sa
		    ON sa.endpoint = st.endpoint AND sa.studio_id = st.id
		 WHERE st.endpoint = ? AND st.is_favourite = 1 AND st.deleted = 0`,
		endpoint, endpoint, endpoint)
	if err != nil {
		return nil, fmt.Errorf("store: favourite aliases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []FavouriteAlias
	for rows.Next() {
		var f FavouriteAlias
		if err := rows.Scan(&f.Kind, &f.ID, &f.Name, &f.Alias, &f.Normalised); err != nil {
			return nil, fmt.Errorf("store: scan favourite alias: %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate favourite aliases: %w", err)
	}
	return out, nil
}

// maxStudiosPerAlias is how many studios may share an alias before it stops
// being treated as naming any of them. A studio's own name is always kept.
const maxStudiosPerAlias = 3

// PruneDerivedAliases drops studio aliases shared by so many studios that they
// name a network rather than a studio.
//
// Every label in a network lists the network's website as its own home, so the
// alias derived from it lands on all of them. Run after a crawl, since that is
// what reintroduces them.
func (s *Store) PruneDerivedAliases(ctx context.Context, endpoint string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM studio_aliases
		 WHERE endpoint = ?
		   AND (endpoint, normalised) IN (
			SELECT endpoint, normalised FROM studio_aliases
			 WHERE endpoint = ?
			 GROUP BY endpoint, normalised HAVING count(DISTINCT studio_id) > ?
		   )
		   AND alias <> (
			SELECT s.name FROM studios s
			 WHERE s.endpoint = studio_aliases.endpoint AND s.id = studio_aliases.studio_id
		   )`, endpoint, endpoint, maxStudiosPerAlias)
	if err != nil {
		return fmt.Errorf("store: prune derived aliases: %w", err)
	}
	return nil
}

// RebuildAliasFreq recomputes how many entities answer to each alias.
//
// Called after a crawl: the counts are derived entirely from the catalogue, so
// they are correct exactly when it stops changing, and stale by at most one
// sync otherwise.
func (s *Store) RebuildAliasFreq(ctx context.Context, endpoint string) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM alias_freq WHERE endpoint = ?`, endpoint); err != nil {
			return fmt.Errorf("store: clear alias frequencies: %w", err)
		}
		counts := []struct {
			kind  string
			query string
			args  []any
		}{
			{
				kind: KindPerformer,
				query: `
					INSERT INTO alias_freq (endpoint, kind, normalised, entities)
					SELECT ?, ?, normalised, count(DISTINCT performer_id) FROM (
						SELECT normalised, performer_id FROM performer_aliases WHERE endpoint = ?
						UNION
						SELECT normalised, performer_id FROM scene_performers WHERE endpoint = ?
					) GROUP BY normalised`,
				args: []any{endpoint, KindPerformer, endpoint, endpoint},
			},
			{
				kind: KindStudio,
				query: `
					INSERT INTO alias_freq (endpoint, kind, normalised, entities)
					SELECT ?, ?, normalised, count(DISTINCT studio_id) FROM studio_aliases
					 WHERE endpoint = ? GROUP BY normalised`,
				args: []any{endpoint, KindStudio, endpoint},
			},
			{
				kind: KindScene,
				query: `
					INSERT INTO alias_freq (endpoint, kind, normalised, entities)
					SELECT ?, ?, title_normalised, count(*) FROM scenes
					 WHERE endpoint = ? AND title_normalised <> '' GROUP BY title_normalised`,
				args: []any{endpoint, KindScene, endpoint},
			},
		}
		for _, c := range counts {
			if _, err := tx.ExecContext(ctx, c.query, c.args...); err != nil {
				return fmt.Errorf("store: count %s aliases: %w", c.kind, err)
			}
		}
		return nil
	})
}

// SearchScenes returns the scenes whose titles most resemble the given text,
// best first, with the bm25 relevance of each.
//
// The query is the text's tokens OR-ed together rather than AND-ed: a release
// reorders a title, drops half of it, prepends the performer's name and
// misspells a word, and an AND of its tokens answers nothing at all. OR-ed,
// every token that does line up contributes and bm25 ranks by how much.
func (s *Store) SearchScenes(ctx context.Context, endpoint, text string, limit int) ([]SceneMatch, error) {
	query := ftsQuery(text)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT scene_id, bm25(scene_search)
		  FROM scene_search
		 WHERE scene_search MATCH ? AND endpoint = ?
		 ORDER BY bm25(scene_search)
		 LIMIT ?`, query, endpoint, limit)
	if err != nil {
		return nil, fmt.Errorf("store: search scenes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []SceneMatch
	for rows.Next() {
		var m SceneMatch
		if err := rows.Scan(&m.SceneID, &m.Score); err != nil {
			return nil, fmt.Errorf("store: scan scene match: %w", err)
		}
		// bm25 is negative, better the lower. Flipped here so every score in
		// this package means the same thing: more is better.
		m.Score = -m.Score
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate scene matches: %w", err)
	}
	return out, nil
}

// SceneMatch is a scene the search returned, with its relevance.
type SceneMatch struct {
	SceneID string
	Score   float64
}

// minToken is the shortest token worth searching for. The trigram tokenizer
// indexes three characters at a time, so anything shorter matches nothing.
const minToken = 3

// ftsQuery builds an OR query out of a text's usable tokens.
func ftsQuery(text string) string {
	var parts []string
	seen := map[string]bool{}
	for _, word := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) {
		if len(word) < minToken || seen[word] {
			continue
		}
		seen[word] = true
		parts = append(parts, `"`+word+`"`)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " OR ")
}

// ScenesOn returns scenes released on a date by any of these studios, or
// featuring any of these performers.
//
// This is the probe that needs no text at all. A release announced today
// carries a date and a site, and one scene usually answers to both — which is
// an identification the title could not supply, least of all when the title is
// mangled or absent.
func (s *Store) ScenesOn(ctx context.Context, endpoint, date string, studioIDs, performerIDs []string) ([]SceneFacts, error) {
	if date == "" || (len(studioIDs) == 0 && len(performerIDs) == 0) {
		return nil, nil
	}

	var ids []string
	if len(studioIDs) > 0 {
		args := []any{endpoint, date}
		for _, id := range studioIDs {
			args = append(args, id)
		}
		found, err := s.ids(ctx, `
			SELECT id FROM scenes
			 WHERE endpoint = ? AND release_date = ? AND deleted = 0
			   AND studio_id IN (`+placeholders(len(studioIDs))+`)`, args...)
		if err != nil {
			return nil, fmt.Errorf("store: scenes on date by studio: %w", err)
		}
		ids = append(ids, found...)
	}
	if len(performerIDs) > 0 {
		args := []any{endpoint, date}
		for _, id := range performerIDs {
			args = append(args, id)
		}
		found, err := s.ids(ctx, `
			SELECT DISTINCT s.id FROM scenes s
			  JOIN scene_performers sp ON sp.endpoint = s.endpoint AND sp.scene_id = s.id
			 WHERE s.endpoint = ? AND s.release_date = ? AND s.deleted = 0
			   AND sp.performer_id IN (`+placeholders(len(performerIDs))+`)`, args...)
		if err != nil {
			return nil, fmt.Errorf("store: scenes on date by performer: %w", err)
		}
		ids = append(ids, found...)
	}
	return s.Scenes(ctx, endpoint, ids)
}

func unique(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
