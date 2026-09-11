package store_test

import (
	"context"
	"testing"

	"homelab/stashtrap/internal/store"
	"homelab/stashtrap/internal/types"
)

// The lookups assemble one thing: a list of bind parameters, sized by a count.
// Everything else is a literal. This proves the values behind those parameters
// stay values.
//
// Probes come from release names off a tracker and ids come from a third-party
// API, so neither is trusted input. A value that reached the query as text
// rather than as a parameter would run here.
func TestLookupsBindHostileValues(t *testing.T) {
	t.Parallel()

	st, _ := open(t)
	ctx := context.Background()
	catalogue(t, st)

	hostile := []string{
		`x'); DROP TABLE scenes;--`,
		`' OR '1'='1`,
		`x", "y`,
	}

	for _, value := range hostile {
		if hits, err := st.Lookup(ctx, endpoint, []string{value}); err != nil {
			t.Fatalf("Lookup(%q): %v", value, err)
		} else if len(hits) != 0 {
			t.Fatalf("Lookup(%q) matched %+v; the value was not bound", value, hits)
		}

		performers, studios, err := st.Favourites(ctx, endpoint, []string{value}, []string{value})
		if err != nil {
			t.Fatalf("Favourites(%q): %v", value, err)
		}
		if len(performers) != 0 || len(studios) != 0 {
			t.Fatalf("Favourites(%q) matched %v/%v", value, performers, studios)
		}

		if scenes, err := st.Scenes(ctx, endpoint, []string{value}); err != nil {
			t.Fatalf("Scenes(%q): %v", value, err)
		} else if len(scenes) != 0 {
			t.Fatalf("Scenes(%q) matched %+v", value, scenes)
		}

		if scenes, err := st.ScenesOn(ctx, endpoint, "2026-09-09", []string{value}, []string{value}); err != nil {
			t.Fatalf("ScenesOn(%q): %v", value, err)
		} else if len(scenes) != 0 {
			t.Fatalf("ScenesOn(%q) matched %+v", value, scenes)
		}

		// The search query is assembled from the text's own tokens, so it is
		// the one most worth pushing quote characters through.
		if _, err := st.SearchScenes(ctx, endpoint, value, 5); err != nil {
			t.Fatalf("SearchScenes(%q): %v", value, err)
		}
	}

	// The catalogue is still there, and still answers.
	performers, studios, scenes, err := st.CatalogueCounts(ctx, endpoint)
	if err != nil {
		t.Fatalf("CatalogueCounts after hostile input: %v", err)
	}
	if performers != 2 || studios != 1 || scenes != 1 {
		t.Fatalf("counts = %d/%d/%d, want 2/1/1", performers, studios, scenes)
	}
}

func TestAnnounceNames(t *testing.T) {
	t.Parallel()

	st, _ := open(t)
	ctx := context.Background()
	for _, name := range []string{"first release", "second release"} {
		record := store.Record{Announce: types.Announce{Name: name, Indexer: "bitporn"}}
		if _, err := st.Insert(ctx, record); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	names, err := st.AnnounceNames(ctx)
	if err != nil {
		t.Fatalf("AnnounceNames: %v", err)
	}
	if len(names) != 2 || names[0] != "first release" {
		t.Fatalf("names = %v, want both in order", names)
	}
}
