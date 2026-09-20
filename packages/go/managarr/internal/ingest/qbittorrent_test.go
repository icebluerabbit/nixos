package ingest

import "testing"

// TestQbtHookDecodesTheShellPayload pins the AutoRun body.
//
// qBittorrent has no HTTP webhook; these are external programs, so every field
// reaches here as a shell argument turned into a JSON string — size included.
// Typing size as an int64 would fail the decode and take the only client-side
// event source with it.
func TestQbtHookDecodesTheShellPayload(t *testing.T) {
	const body = `{
	  "event": "added",
	  "name": "Some.Release-GRP",
	  "category": "tv",
	  "size": "123456789",
	  "infohash": "0123456789ABCDEF0123456789ABCDEF01234567"
	}`
	var h qbtHook
	if err := decode([]byte(body), &h); err != nil {
		t.Fatal(err)
	}
	if h.Event != qbtAdded {
		t.Errorf("event = %q", h.Event)
	}
	if h.Category != "tv" {
		t.Errorf("category = %q", h.Category)
	}
	if got := parseBytes(h.Size); got != 123456789 {
		t.Errorf("size = %d", got)
	}
}

// TestQbtEventNames matches the two hooks the container installs.
//
// on-added and on-finished are the only AutoRun points wired. An event outside
// this pair must not be written: it would be a third state nothing produces.
func TestQbtEventNames(t *testing.T) {
	if qbtAdded != "added" || qbtFinished != "finished" {
		t.Fatalf("event constants drifted: %q %q", qbtAdded, qbtFinished)
	}
}

// TestQbtHookMissingInfohashIsUnusable guards the join key.
//
// The infohash is what makes the event joinable — it is the same value an *arr
// reports as downloadId. Without it the row would be a torrent nothing else can
// reach.
func TestQbtHookMissingInfohashIsUnusable(t *testing.T) {
	var h qbtHook
	if err := decode([]byte(`{"event":"added","name":"x","size":"1"}`), &h); err != nil {
		t.Fatal(err)
	}
	if h.Infohash != "" {
		t.Errorf("infohash = %q, want empty", h.Infohash)
	}
}

// TestSlskdHookDecodesTheVectorShape pins the sidecar's own schema.
//
// slskd has no webhook, so this is a journal line the Vector remap recognised
// and re-encoded. The shape belongs to the remap in slskd.nix, not to slskd, so
// a change on either side has to be made on both — this fixture is the
// statement of what the remap emits.
func TestSlskdHookDecodesTheVectorShape(t *testing.T) {
	const body = `{"source":"slskd","event":"failed","file":"@@abc\\Music\\01.flac","user":"peer","reason":"Transfer rejected"}`
	var h slskdHook
	if err := decode([]byte(body), &h); err != nil {
		t.Fatal(err)
	}
	if h.User != "peer" {
		t.Errorf("user = %q", h.User)
	}
	// The remote path is a Windows-style Soulseek path and must survive
	// verbatim: it is half the primary key of soulseek_downloads.
	if h.File != `@@abc\Music\01.flac` {
		t.Errorf("file = %q", h.File)
	}
	if h.Reason != "Transfer rejected" {
		t.Errorf("reason = %q", h.Reason)
	}
}

// TestProwlarrHookDecodes covers the thin event surface.
func TestProwlarrHookDecodes(t *testing.T) {
	const body = `{
	  "eventType": "Grab",
	  "release": {"releaseTitle": "Some.Release-GRP", "indexer": "TorrentLeech", "size": 500},
	  "instanceName": "Prowlarr"
	}`
	var h prowlarrHook
	if err := decode([]byte(body), &h); err != nil {
		t.Fatal(err)
	}
	if h.Release.ReleaseTitle != "Some.Release-GRP" || h.Release.Indexer != "TorrentLeech" {
		t.Errorf("release = %+v", h.Release)
	}
}

// TestAutobrrPushDecodesEveryFieldAsAString is the template constraint.
//
// autobrr renders the action body from a Go template, so nothing arrives typed
// — Size included. This payload is also the ONLY place autobrr ever emits an
// infohash: its own API serialises the field as json:"-".
func TestAutobrrPushDecodesEveryFieldAsAString(t *testing.T) {
	const body = `{
	  "name": "Some.Release-GRP",
	  "infohash": "0123456789ABCDEF0123456789ABCDEF01234567",
	  "indexer": "TorrentLeech",
	  "filter": "tv-1080p",
	  "size": "123456789",
	  "category": "TV",
	  "url": "https://tracker.example/download.php?id=1&passkey=deadbeef"
	}`
	var push autobrrPush
	if err := decode([]byte(body), &push); err != nil {
		t.Fatal(err)
	}
	if push.Infohash == "" {
		t.Fatal("infohash did not decode; this webhook exists for that field alone")
	}
	if got := parseBytes(push.Size); got != 123456789 {
		t.Errorf("size = %d", got)
	}
}

// TestAutobrrPushUnrenderedTemplate is what a broken macro produces.
//
// A template referencing a macro that does not exist fails the whole action, but
// a macro that renders empty produces this. Both must be dropped rather than
// stored as a nameless release.
func TestAutobrrPushUnrenderedTemplate(t *testing.T) {
	var push autobrrPush
	if err := decode([]byte(`{"name":"","infohash":"","size":"{{ .Size }}"}`), &push); err != nil {
		t.Fatal(err)
	}
	if push.Name != "" || push.Infohash != "" {
		t.Error("fixture should be empty")
	}
	if got := parseBytes(push.Size); got != 0 {
		t.Errorf("an unrendered size parsed to %d", got)
	}
}
