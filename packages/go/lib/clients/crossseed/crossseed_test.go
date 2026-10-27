package crossseed

import (
	"encoding/json"
	"testing"
	"time"
)

// This package is types only — the client that used to live here became
// lib/sidecar's generic one, which is tested there. What is left worth pinning
// is the wire shape: managarr-crossseed marshals these and managarr unmarshals
// them, and the two halves never meet at compile time. A renamed JSON tag would
// leave both sides building and every field silently zero.

func TestDecisionWireShape(t *testing.T) {
	// Field names exactly as cross-seed's own camelCase, which is what the
	// sidecar emits into a sidecar.Event payload.
	const wire = `{
		"searcheeName":"Some.Release.2019.1080p.BluRay-GRP",
		"infoHash":"abc123",
		"guid":"https://tracker.example/download/1?rss=SECRET",
		"decision":"MATCH",
		"firstSeen":"2026-01-02T03:04:05Z",
		"lastSeen":"2026-02-03T04:05:06Z"
	}`

	var got Decision
	if err := json.Unmarshal([]byte(wire), &got); err != nil {
		t.Fatal(err)
	}

	want := Decision{
		SearcheeName: "Some.Release.2019.1080p.BluRay-GRP",
		InfoHash:     "abc123",
		GUID:         "https://tracker.example/download/1?rss=SECRET",
		Decision:     "MATCH",
		FirstSeen:    time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		LastSeen:     time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC),
	}
	if got != want {
		t.Errorf("decision =\n%+v\nwant\n%+v", got, want)
	}

	// Round-trip: what the sidecar writes must be what managarr reads.
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var back Decision
	if err := json.Unmarshal(encoded, &back); err != nil {
		t.Fatal(err)
	}
	if back != want {
		t.Errorf("round trip lost data:\n%+v\nwant\n%+v", back, want)
	}
}

func TestClientTorrentWireShape(t *testing.T) {
	var got []ClientTorrent
	err := json.Unmarshal([]byte(`[
		{"name":"Release.A","infoHash":"aaa"},
		{"name":"Release.A","infoHash":"bbb"}
	]`), &got)
	if err != nil {
		t.Fatal(err)
	}

	// One name, two hashes: they are cross-seeds of each other. The mapping
	// is genuinely one-to-many, and deciding what to do about the ambiguity
	// belongs to the consumer, not to the type.
	if len(got) != 2 {
		t.Fatalf("got %d torrents, want both hashes", len(got))
	}
	if got[0].Name != got[1].Name || got[0].InfoHash == got[1].InfoHash {
		t.Errorf("torrents = %+v", got)
	}
}

func TestStatsWireShape(t *testing.T) {
	var got Stats
	err := json.Unmarshal(
		[]byte(`{"decisions":48210,"searchees":6912,"clientTorrents":7003}`), &got)
	if err != nil {
		t.Fatal(err)
	}

	want := Stats{Decisions: 48210, Searchees: 6912, ClientTorrents: 7003}
	if got != want {
		t.Errorf("stats = %+v, want %+v", got, want)
	}
}

// The sidecar builds its routes from these constants and managarr calls them
// from the same constants, so they cannot drift — but only as long as both
// sides actually use them rather than a literal.
func TestRouteConstants(t *testing.T) {
	if Source != "cross-seed" {
		t.Errorf("Source = %q; it must match the infra client name", Source)
	}
	if KindDecision != "decision" {
		t.Errorf("KindDecision = %q", KindDecision)
	}
	for _, p := range []string{ClientTorrentsPath, StatsPath} {
		if p == "" || p[0] != '/' {
			t.Errorf("route %q must be an absolute path", p)
		}
	}
}
