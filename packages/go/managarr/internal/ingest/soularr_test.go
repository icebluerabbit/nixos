package ingest

import "testing"

// TestSoularrHookDecodesTheVectorShape pins the sidecar's own schema.
//
// soularr has no webhook, so this is a journal line the Vector remap recognised
// and re-encoded. The shape belongs to _notify.nix, not to soularr, so a change
// on either side has to be made on both — this fixture is the statement of what
// the remap emits.
func TestSoularrHookDecodesTheVectorShape(t *testing.T) {
	const body = `{"source":"soularr","event":"import_failed","album":"Some Album","artist":"Some Artist","count":null,"album_id":"1234"}`
	var h soularrHook
	if err := decode([]byte(body), &h); err != nil {
		t.Fatal(err)
	}
	if h.Event != "import_failed" {
		t.Errorf("event = %q", h.Event)
	}
	// The only field that makes any of this storable.
	if h.AlbumID != "1234" {
		t.Errorf("album_id = %q", h.AlbumID)
	}
}

// TestSoularrHookWithoutAnAlbumIDIsUnusable is the rule, not an edge case.
//
// soularr logs a Lidarr album id on exactly one of its lines. Its other
// outcomes — search failed, download completed — carry album and artist as free
// text, and resolving those to content would mean matching on title, which this
// model does nowhere.
func TestSoularrHookWithoutAnAlbumIDIsUnusable(t *testing.T) {
	for _, body := range []string{
		`{"source":"soularr","event":"search_failed","album":"A","artist":"B"}`,
		`{"source":"soularr","event":"downloaded","album":"A","artist":"B"}`,
		`{"source":"soularr","event":"grabbed","count":"3"}`,
	} {
		var h soularrHook
		if err := decode([]byte(body), &h); err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		if h.AlbumID != "" {
			t.Errorf("%s unexpectedly carries an album id", body)
		}
	}
}

// TestSoularrUnusableEventsAreSilent covers the handler's guards.
//
// A nil dao would panic if any of these reached a write, which is the point:
// none of them must.
func TestSoularrUnusableEventsAreSilent(t *testing.T) {
	i := New(nil, nil, discard(), Config{Lidarr: "lidarr"})
	for _, body := range []string{
		`{"source":"soularr","event":"search_failed","album":"A","artist":"B"}`,
		`{"source":"soularr","event":"","album_id":"1"}`,
		`not json`,
		`{}`,
	} {
		i.soularr(t.Context(), []byte(body))
	}

	// And with no Lidarr configured there is no catalog to resolve against.
	noLidarr := New(nil, nil, discard(), Config{})
	noLidarr.soularr(t.Context(),
		[]byte(`{"source":"soularr","event":"import_failed","album_id":"1"}`))
}
