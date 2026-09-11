package store_test

import (
	"context"
	"testing"

	"homelab/stashtrap/internal/store"
	"homelab/stashtrap/internal/types"
)

// catalogue seeds one studio, two performers and a scene joining them.
func catalogue(t *testing.T, st *store.Store) {
	t.Helper()
	ctx := context.Background()

	if err := st.UpsertStudio(ctx, endpoint, store.Studio{
		ID: "s1", Name: "ExampleStudio", Aliases: []string{"examplestudio"}, IsFavourite: true,
	}); err != nil {
		t.Fatalf("UpsertStudio: %v", err)
	}
	if err := st.UpsertPerformer(ctx, endpoint, store.Performer{
		ID: "p1", Name: "Jane Doe", Aliases: []string{"jane.doe"}, IsFavourite: true,
	}); err != nil {
		t.Fatalf("UpsertPerformer: %v", err)
	}
	if err := st.UpsertPerformer(ctx, endpoint, store.Performer{
		ID: "p2", Name: "Second Performer",
	}); err != nil {
		t.Fatalf("UpsertPerformer: %v", err)
	}
	if err := st.UpsertScene(ctx, endpoint, store.Scene{
		ID: "sc1", Title: "A Long And Specific Title", StudioID: "s1", ReleaseDate: "2026-09-09",
		Performers: []store.ScenePerformer{
			{PerformerID: "p1", As: "Jane Doe"},
			{PerformerID: "p2", As: "Second Performer"},
		},
	}); err != nil {
		t.Fatalf("UpsertScene: %v", err)
	}
}

func TestLookupFindsEveryKind(t *testing.T) {
	t.Parallel()

	st, _ := open(t)
	catalogue(t, st)

	hits, err := st.Lookup(context.Background(), endpoint,
		[]string{"examplestudio", "janedoe", "alongandspecifictitle", "nothing"})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}

	byKind := map[string][]store.Hit{}
	for _, h := range hits {
		byKind[h.Kind] = append(byKind[h.Kind], h)
	}
	for _, kind := range []string{store.KindStudio, store.KindPerformer, store.KindScene} {
		if len(byKind[kind]) == 0 {
			t.Fatalf("no %s hit in %+v", kind, hits)
		}
	}
	// Nothing may answer to a probe the catalogue does not know.
	for _, h := range hits {
		if h.Probe == "nothing" {
			t.Fatalf("unknown probe matched: %+v", h)
		}
	}
}

// The share of an alias is what specificity is computed from, so it has to
// count the entities answering to it rather than the rows.
func TestLookupReportsSharedCount(t *testing.T) {
	t.Parallel()

	st, _ := open(t)
	ctx := context.Background()
	catalogue(t, st)

	// A second performer under the same alias.
	if err := st.UpsertPerformer(ctx, endpoint, store.Performer{
		ID: "p3", Name: "Jane Doe", Aliases: []string{"jane.doe"},
	}); err != nil {
		t.Fatalf("UpsertPerformer: %v", err)
	}
	if err := st.RebuildAliasFreq(ctx, endpoint); err != nil {
		t.Fatalf("RebuildAliasFreq: %v", err)
	}

	hits, err := st.Lookup(ctx, endpoint, []string{"janedoe"})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("no hits")
	}
	for _, h := range hits {
		if h.Kind == store.KindPerformer && h.Shared != 2 {
			t.Fatalf("shared = %d for an alias two performers answer to: %+v", h.Shared, h)
		}
	}
}

// What a release was found to name is stored with it, so a grab stays
// explicable and a rejected release can be reconsidered.
func TestInsertRecordsTheReading(t *testing.T) {
	t.Parallel()

	st, _ := open(t)
	ctx := context.Background()
	catalogue(t, st)

	id, err := st.Insert(ctx, store.Record{
		Announce: types.Announce{Name: "[ExampleStudio] Jane Doe - A Title", Indexer: "bitporn"},
		Accepted: true,
		Reason:   "favourite-performer",
		Reading: store.Reading{
			SceneID:      "sc1",
			Score:        7.5,
			Corroborated: true,
			Matches: []store.RecordedMatch{
				{Kind: store.KindPerformer, EntityID: "p1", Probe: "jane doe", Favourite: true},
				{Kind: store.KindStudio, EntityID: "s1", Probe: "examplestudio", Favourite: true},
				{Kind: store.KindPerformer, EntityID: "p2", Probe: "second performer"},
				// The same entity found twice in one name is one fact.
				{Kind: store.KindPerformer, EntityID: "p1", Probe: "jane doe", Favourite: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Rejected, so it shows up as something that could now be taken.
	if _, err := st.Insert(ctx, store.Record{
		Announce: types.Announce{Name: "[ExampleStudio] Jane Doe - Another", Indexer: "bitporn"},
		Reason:   "record-only",
		Reading: store.Reading{Matches: []store.RecordedMatch{
			{Kind: store.KindPerformer, EntityID: "p1", Probe: "jane doe", Favourite: true},
		}},
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	pending, err := st.UngrabbedFavourites(ctx)
	if err != nil {
		t.Fatalf("UngrabbedFavourites: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending = %+v, want only the rejected one", pending)
	}
	if pending[0].ID == id {
		t.Fatalf("an accepted release was listed as ungrabbed: %+v", pending[0])
	}
	if len(pending[0].Entities) != 1 || pending[0].Entities[0] != "p1" {
		t.Fatalf("entities = %v, want [p1]", pending[0].Entities)
	}
}

func TestFavouritesNarrowsToFollowed(t *testing.T) {
	t.Parallel()

	st, _ := open(t)
	catalogue(t, st)

	performers, studios, err := st.Favourites(context.Background(), endpoint,
		[]string{"p1", "p2"}, []string{"s1"})
	if err != nil {
		t.Fatalf("Favourites: %v", err)
	}
	if len(performers) != 1 || performers[0] != "p1" {
		t.Fatalf("performers = %v, want only the followed one", performers)
	}
	if len(studios) != 1 || studios[0] != "s1" {
		t.Fatalf("studios = %v, want [s1]", studios)
	}
}

// Ids identify entities to a database and to nobody else, so logs and panels
// need the names behind them.
func TestEntityNames(t *testing.T) {
	t.Parallel()

	st, _ := open(t)
	catalogue(t, st)

	names, err := st.EntityNames(context.Background(), endpoint,
		[]string{"p1", "p2", "missing"}, []string{"s1"})
	if err != nil {
		t.Fatalf("EntityNames: %v", err)
	}
	for id, want := range map[string]string{
		"p1": "Jane Doe",
		"p2": "Second Performer",
		"s1": "ExampleStudio",
	} {
		if names[id] != want {
			t.Fatalf("names[%q] = %q, want %q", id, names[id], want)
		}
	}
	if _, found := names["missing"]; found {
		t.Fatalf("an unknown id was given a name: %v", names)
	}
}

func TestFavouriteAliases(t *testing.T) {
	t.Parallel()

	st, _ := open(t)
	catalogue(t, st)

	aliases, err := st.FavouriteAliases(context.Background(), endpoint)
	if err != nil {
		t.Fatalf("FavouriteAliases: %v", err)
	}

	got := map[string]bool{}
	for _, a := range aliases {
		got[a.Normalised] = true
		if a.ID == "p2" {
			t.Fatalf("alias of an unfollowed performer returned: %+v", a)
		}
	}
	for _, want := range []string{"janedoe", "examplestudio"} {
		if !got[want] {
			t.Fatalf("aliases %v missing %q", got, want)
		}
	}
}

func TestSearchScenesRanksByRelevance(t *testing.T) {
	t.Parallel()

	st, _ := open(t)
	ctx := context.Background()
	catalogue(t, st)
	if err := st.UpsertScene(ctx, endpoint, store.Scene{
		ID: "sc2", Title: "Something Else Entirely", StudioID: "s1",
	}); err != nil {
		t.Fatalf("UpsertScene: %v", err)
	}

	// Reordered and partial, which is how releases write titles.
	found, err := st.SearchScenes(ctx, endpoint, "specific title long", 5)
	if err != nil {
		t.Fatalf("SearchScenes: %v", err)
	}
	if len(found) == 0 || found[0].SceneID != "sc1" {
		t.Fatalf("search returned %+v, want sc1 first", found)
	}
	if found[0].Score <= 0 {
		t.Fatalf("score = %v, want a positive relevance", found[0].Score)
	}
}

func TestScenesOnDate(t *testing.T) {
	t.Parallel()

	st, _ := open(t)
	ctx := context.Background()
	catalogue(t, st)

	byStudio, err := st.ScenesOn(ctx, endpoint, "2026-09-09", []string{"s1"}, nil)
	if err != nil {
		t.Fatalf("ScenesOn: %v", err)
	}
	if len(byStudio) != 1 || byStudio[0].ID != "sc1" {
		t.Fatalf("by studio = %+v, want sc1", byStudio)
	}
	if len(byStudio[0].Performers) != 2 {
		t.Fatalf("performers = %v, want the scene's cast", byStudio[0].Performers)
	}

	byPerformer, err := st.ScenesOn(ctx, endpoint, "2026-09-09", nil, []string{"p1"})
	if err != nil {
		t.Fatalf("ScenesOn: %v", err)
	}
	if len(byPerformer) != 1 {
		t.Fatalf("by performer = %+v, want sc1", byPerformer)
	}

	// A different day is a different scene.
	none, err := st.ScenesOn(ctx, endpoint, "2020-01-01", []string{"s1"}, nil)
	if err != nil {
		t.Fatalf("ScenesOn: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("wrong date returned %+v", none)
	}
}

// An alias shared by a network's labels names the network, not any label, so
// it is dropped from all of them — except where it is a studio's own name.
func TestPruneDerivedAliases(t *testing.T) {
	t.Parallel()

	st, _ := open(t)
	ctx := context.Background()

	// A network and five labels, each carrying the network's domain.
	if err := st.UpsertStudio(ctx, endpoint, store.Studio{
		ID: "net", Name: "network", Aliases: []string{"network"},
	}); err != nil {
		t.Fatalf("UpsertStudio: %v", err)
	}
	for _, id := range []string{"l1", "l2", "l3", "l4", "l5"} {
		if err := st.UpsertStudio(ctx, endpoint, store.Studio{
			ID: id, Name: "Label " + id, Aliases: []string{"network"},
		}); err != nil {
			t.Fatalf("UpsertStudio: %v", err)
		}
	}

	if err := st.PruneDerivedAliases(ctx, endpoint); err != nil {
		t.Fatalf("PruneDerivedAliases: %v", err)
	}

	hits, err := st.Lookup(ctx, endpoint, []string{"network"})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	var studios []string
	for _, h := range hits {
		if h.Kind == store.KindStudio {
			studios = append(studios, h.ID)
		}
	}
	if len(studios) != 1 || studios[0] != "net" {
		t.Fatalf("studios answering to the network name = %v, want only the network", studios)
	}
}
