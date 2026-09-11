package store

import (
	"context"

	"homelab/stashtrap/internal/parse"
)

// Catalogue adapts a Store to the lookups the parser needs, bound to one
// stash-box namespace.
//
// The translation is deliberate rather than incidental: the parser does not
// import the store, so that scoring can be tested without a database, and the
// store does not know what a reading is. This is the one place the two meet.
type Catalogue struct {
	store    *Store
	endpoint string
}

// NewCatalogue binds a store to one stash-box namespace.
func (s *Store) NewCatalogue(endpoint string) *Catalogue {
	return &Catalogue{store: s, endpoint: endpoint}
}

// Lookup matches every probe against every kind of catalogue entry.
func (c *Catalogue) Lookup(ctx context.Context, probes []string) ([]parse.Hit, error) {
	hits, err := c.store.Lookup(ctx, c.endpoint, probes)
	if err != nil {
		return nil, err
	}
	out := make([]parse.Hit, 0, len(hits))
	for _, h := range hits {
		out = append(out, parse.Hit{
			Probe:  h.Probe,
			Kind:   h.Kind,
			ID:     h.ID,
			Source: h.Source,
			Shared: h.Shared,
		})
	}
	return out, nil
}

// Scenes returns the relations of these scenes.
func (c *Catalogue) Scenes(ctx context.Context, ids []string) ([]parse.Scene, error) {
	facts, err := c.store.Scenes(ctx, c.endpoint, ids)
	if err != nil {
		return nil, err
	}
	return scenesFrom(facts), nil
}

// ScenesOn returns scenes released on a date by these studios or with these
// performers.
func (c *Catalogue) ScenesOn(ctx context.Context, date string, studioIDs, performerIDs []string) ([]parse.Scene, error) {
	facts, err := c.store.ScenesOn(ctx, c.endpoint, parse.ISODate(date), studioIDs, performerIDs)
	if err != nil {
		return nil, err
	}
	return scenesFrom(facts), nil
}

// SearchScenes returns the scenes whose titles most resemble text.
func (c *Catalogue) SearchScenes(ctx context.Context, text string, limit int) ([]parse.SceneMatch, error) {
	found, err := c.store.SearchScenes(ctx, c.endpoint, text, limit)
	if err != nil {
		return nil, err
	}
	out := make([]parse.SceneMatch, 0, len(found))
	for _, m := range found {
		out = append(out, parse.SceneMatch{SceneID: m.SceneID, Score: m.Score})
	}
	return out, nil
}

func scenesFrom(facts []SceneFacts) []parse.Scene {
	out := make([]parse.Scene, 0, len(facts))
	for _, f := range facts {
		out = append(out, parse.Scene{
			ID:          f.ID,
			Title:       f.Title,
			StudioID:    f.StudioID,
			ParentID:    f.ParentID,
			ReleaseDate: f.ReleaseDate,
			Performers:  f.Performers,
		})
	}
	return out
}
