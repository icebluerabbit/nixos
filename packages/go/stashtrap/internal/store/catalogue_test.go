package store_test

import (
	"context"
	"testing"

	"homelab/stashtrap/internal/store"
)

const endpoint = "https://stashdb.org/graphql"

func TestNormalise(t *testing.T) {
	t.Parallel()

	// The whole point: every spelling a release name might use has to fold
	// to one key.
	same := []string{"jane.doe", "Jane Doe", "jane_doe", "JANE-DOE", "Jane  Doe!"}
	want := store.Normalise(same[0])
	for _, s := range same[1:] {
		if got := store.Normalise(s); got != want {
			t.Fatalf("Normalise(%q) = %q, want %q", s, got, want)
		}
	}
	if want != "janedoe" {
		t.Fatalf("Normalise = %q, want janedoe", want)
	}
	if got := store.Normalise("Some! Studio 4K"); got != "somestudio4k" {
		t.Fatalf("Normalise kept punctuation: %q", got)
	}
}

func TestUpsertPerformer(t *testing.T) {
	t.Parallel()

	st, _ := open(t)
	ctx := context.Background()

	p := store.Performer{
		ID:          "p1",
		Name:        "Jane Doe",
		Aliases:     []string{"jane.doe", "JaneD"},
		URLs:        []store.CatalogueURL{{URL: "https://example.invalid/p1", Site: "ExampleSite"}},
		IsFavourite: true,
	}
	if err := st.UpsertPerformer(ctx, endpoint, p); err != nil {
		t.Fatalf("UpsertPerformer: %v", err)
	}

	ids, err := st.FavouritePerformerIDs(ctx, endpoint)
	if err != nil || len(ids) != 1 || ids[0] != "p1" {
		t.Fatalf("FavouritePerformerIDs = %v, %v", ids, err)
	}

	// Re-upserting with a shorter alias list must drop the removed alias:
	// stash-box is the source of truth, and a stale alias keeps matching
	// releases that are no longer theirs.
	p.Aliases = []string{"JaneD"}
	if err := st.UpsertPerformer(ctx, endpoint, p); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}

	perf, studios, scenes, err := st.CatalogueCounts(ctx, endpoint)
	if err != nil {
		t.Fatalf("CatalogueCounts: %v", err)
	}
	if perf != 1 || studios != 0 || scenes != 0 {
		t.Fatalf("counts = %d/%d/%d, want 1/0/0", perf, studios, scenes)
	}
}

func TestUpsertStudioAndScene(t *testing.T) {
	t.Parallel()

	st, _ := open(t)
	ctx := context.Background()

	if err := st.UpsertStudio(ctx, endpoint, store.Studio{
		ID:          "s1",
		Name:        "ExampleStudio",
		ParentID:    "net1",
		Aliases:     []string{"ExampleStudio.com"},
		IsFavourite: true,
	}); err != nil {
		t.Fatalf("UpsertStudio: %v", err)
	}

	if err := st.UpsertScene(ctx, endpoint, store.Scene{
		ID:          "sc1",
		Title:       "A Scene",
		ReleaseDate: "2026-09-01",
		Code:        "TS123",
		Duration:    1800,
		StudioID:    "s1",
		Performers:  []store.ScenePerformer{{PerformerID: "p1", As: "Jane Doe"}},
		Tags:        []store.Tag{{ID: "t1", Name: "Vintage"}},
		URLs:        []store.CatalogueURL{{URL: "https://example.invalid/sc1", Site: "ExampleStudio"}},
		Fingerprints: []store.Fingerprint{
			{Algorithm: "OSHASH", Hash: "abc", Duration: 1800, Submissions: 3},
		},
	}); err != nil {
		t.Fatalf("UpsertScene: %v", err)
	}

	ids, err := st.FavouriteStudioIDs(ctx, endpoint)
	if err != nil || len(ids) != 1 || ids[0] != "s1" {
		t.Fatalf("FavouriteStudioIDs = %v, %v", ids, err)
	}

	// Idempotent: the same scene twice is one row, not two, and the joins
	// are replaced rather than duplicated.
	if err := st.UpsertScene(ctx, endpoint, store.Scene{ID: "sc1", StudioID: "s1"}); err != nil {
		t.Fatalf("re-upsert scene: %v", err)
	}
	_, _, scenes, err := st.CatalogueCounts(ctx, endpoint)
	if err != nil || scenes != 1 {
		t.Fatalf("scenes = %d, %v; want 1", scenes, err)
	}
}

// A second stash-box is a different namespace: the same id in both must not
// collide, which is why endpoint is in every primary key.
func TestEndpointNamespacesIDs(t *testing.T) {
	t.Parallel()

	st, _ := open(t)
	ctx := context.Background()
	const other = "https://fansdb.cc/graphql"

	for _, ep := range []string{endpoint, other} {
		if err := st.UpsertPerformer(ctx, ep, store.Performer{
			ID: "shared", Name: "Same Id", IsFavourite: true,
		}); err != nil {
			t.Fatalf("upsert on %s: %v", ep, err)
		}
	}

	for _, ep := range []string{endpoint, other} {
		n, _, _, err := st.CatalogueCounts(ctx, ep)
		if err != nil || n != 1 {
			t.Fatalf("%s: performers = %d, %v; want 1", ep, n, err)
		}
	}
}

func TestSyncState(t *testing.T) {
	t.Parallel()

	st, _ := open(t)
	ctx := context.Background()

	if err := st.StartSync(ctx, endpoint, "performers"); err != nil {
		t.Fatalf("StartSync: %v", err)
	}
	if err := st.FinishSync(ctx, endpoint, "performers", 42, nil); err != nil {
		t.Fatalf("FinishSync: %v", err)
	}
	// A failed crawl keeps its error, so a stale catalogue is visible.
	if err := st.StartSync(ctx, endpoint, "scenes"); err != nil {
		t.Fatalf("StartSync scenes: %v", err)
	}
	if err := st.FinishSync(ctx, endpoint, "scenes", 7, context.Canceled); err != nil {
		t.Fatalf("FinishSync scenes: %v", err)
	}
}
