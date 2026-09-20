package autobrr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"homelab/httpx"
)

func newTestClient(t *testing.T, h http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := New(httpx.Config{
		BaseURL:  srv.URL,
		RetryMax: -1,
		Headers:  http.Header{"X-API-Token": []string{"test-token"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestReleasesPagesByOffset pins the paging mechanism.
//
// autobrr reports a next_cursor and it is NOT usable: passing it back as
// ?cursor= returns an empty page, including when it is the exact value autobrr
// just supplied (verified against 1.82.1). A walk driven by the cursor stops
// after one page and reports success, which looks identical to "there were only
// a thousand releases".
func TestReleasesPagesByOffset(t *testing.T) {
	const pageSize = 2
	var offsets []int

	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		off, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		offsets = append(offsets, off)

		if got := r.URL.Query().Get("limit"); got != strconv.Itoa(pageSize) {
			t.Errorf("limit = %q", got)
		}

		var data []Release
		switch off {
		case 0:
			data = []Release{{ID: 1}, {ID: 2}}
		case 2:
			data = []Release{{ID: 3}} // short page: the last one
		default:
			t.Errorf("unexpected offset %d", off)
		}
		// next_cursor is deliberately a value that would break the walk if
		// anything used it.
		_ = json.NewEncoder(w).Encode(releasePage{Data: data, NextCursor: 999, Count: 3})
	}))

	var seen []int64
	err := c.Releases(context.Background(), pageSize, func(page []Release) bool {
		for _, r := range page {
			seen = append(seen, r.ID)
		}
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offsets) != 2 || offsets[0] != 0 || offsets[1] != 2 {
		t.Errorf("offsets = %v, want [0 2]", offsets)
	}
	if len(seen) != 3 {
		t.Errorf("saw %d releases, want 3", len(seen))
	}
}

// TestReleasesStopsWhenTheCallbackSaysSo is how an incremental pass ends.
//
// The walk is newest-first, so a pass with little new work reaches releases it
// already has on the first page. Ignoring the stop signal would make every
// scheduled pass a full 127,000-row walk.
func TestReleasesStopsWhenTheCallbackSaysSo(t *testing.T) {
	var requests int
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_ = json.NewEncoder(w).Encode(releasePage{
			Data: []Release{{ID: 1}, {ID: 2}}, Count: 100,
		})
	}))

	err := c.Releases(context.Background(), 2, func([]Release) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Errorf("made %d requests after being told to stop, want 1", requests)
	}
}

// TestReleasesEmptyPageEndsTheWalk guards against an infinite loop.
func TestReleasesEmptyPageEndsTheWalk(t *testing.T) {
	var requests int
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests > 5 {
			t.Fatal("the walk did not terminate on an empty page")
		}
		_ = json.NewEncoder(w).Encode(releasePage{Data: nil, Count: 0})
	}))

	var called bool
	if err := c.Releases(context.Background(), 10, func([]Release) bool {
		called = true
		return true
	}); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("the callback ran for an empty page")
	}
	if requests != 1 {
		t.Errorf("made %d requests", requests)
	}
}

// TestReleaseHasNoInfohashField is the constraint the whole webhook exists for.
//
// autobrr's domain model tags the infohash `json:"-"`, so no API response ever
// carries it. If a field named infohash appeared here, the managarr filter
// action — and the only path by which an announce is ever tied to a torrent —
// would be redundant, and its absence is worth asserting rather than
// remembering.
func TestReleaseHasNoInfohashField(t *testing.T) {
	body, err := json.Marshal(Release{ID: 1, Name: "Some.Release-GRP"})
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]any
	if err := json.Unmarshal(body, &generic); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"info_hash", "infohash", "torrent_hash"} {
		if _, ok := generic[key]; ok {
			t.Errorf("Release serialises %q; the webhook may no longer be needed", key)
		}
	}
	// normalized_hash is present and is NOT an infohash — it is autobrr's
	// dedupe digest over the parsed release name.
	if _, ok := generic["normalized_hash"]; !ok {
		t.Error("normalized_hash disappeared")
	}
}

// TestReleaseDecodesTheParsedFields covers the half of the payload that names
// what the release actually is.
func TestReleaseDecodesTheParsedFields(t *testing.T) {
	const body = `{
	  "id": 7,
	  "filter_status": "FILTER_APPROVED",
	  "rejections": [],
	  "indexer": {"id": 3, "name": "TorrentLeech", "identifier": "torrentleech"},
	  "filter": "tv-1080p",
	  "protocol": "torrent",
	  "timestamp": "2026-09-13T12:00:00Z",
	  "announce_type": "NEW",
	  "name": "Some.Show.S01E01.1080p.WEB-DL.H264-GRP",
	  "normalized_hash": "abc123",
	  "size": 123456789,
	  "category": "TV :: Episodes HD",
	  "season": 1, "episode": 1, "year": 2026,
	  "resolution": "1080p", "source": "WEB-DL",
	  "codec": ["H.264"], "container": "mkv", "hdr": ["HDR10"],
	  "group": "GRP", "proper": false, "repack": true,
	  "uploader": "someone", "pre_time": "3m 20s",
	  "action_status": [
	    {"status": "PUSH_APPROVED", "action": "sonarr", "type": "SONARR",
	     "rejections": [], "timestamp": "2026-09-13T12:00:05Z"},
	    {"status": "PUSH_REJECTED", "action": "radarr", "type": "RADARR",
	     "rejections": ["Unknown Movie"], "timestamp": "2026-09-13T12:00:06Z"}
	  ]
	}`
	var r Release
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatal(err)
	}

	if r.Indexer.Name != "TorrentLeech" {
		t.Errorf("indexer = %+v", r.Indexer)
	}
	if r.FilterName != "tv-1080p" {
		t.Errorf(`filter decoded from the wrong key: %q`, r.FilterName)
	}
	if r.Resolution != "1080p" || r.Source != "WEB-DL" || r.Group != "GRP" {
		t.Errorf("parsed fields = %q %q %q", r.Resolution, r.Source, r.Group)
	}
	if !r.Repack || r.Proper {
		t.Errorf("proper=%v repack=%v", r.Proper, r.Repack)
	}
	if len(r.Codec) != 1 || len(r.HDR) != 1 {
		t.Errorf("codec=%v hdr=%v; both are arrays, not strings", r.Codec, r.HDR)
	}
	if len(r.ActionStatus) != 2 {
		t.Fatalf("got %d actions", len(r.ActionStatus))
	}
	// The rejection text is the only record of why an approved release never
	// reached an *arr.
	if len(r.ActionStatus[1].Rejections) != 1 {
		t.Errorf("rejections = %v", r.ActionStatus[1].Rejections)
	}
	if r.Timestamp.IsZero() {
		t.Error("timestamp did not decode")
	}
}

// TestNetworksDecodesChannels covers the IRC half.
func TestNetworksDecodesChannels(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{
		  "id": 1, "name": "TorrentLeech", "server": "irc.torrentleech.org",
		  "port": 7011, "tls": true, "enabled": true, "nick": "bot",
		  "channels": [
		    {"id": 0, "name": "#tlannounces", "enabled": true, "monitoring": true},
		    {"id": 0, "name": "#announce", "enabled": false, "monitoring": false}
		  ]
		}]`))
	}))

	nets, err := c.Networks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 1 {
		t.Fatalf("got %d networks", len(nets))
	}
	// Several channels carry id 0 — autobrr does not assign one until the
	// channel has been joined. Keying on the id would collide them into one
	// row, which it did.
	if len(nets[0].Channels) != 2 {
		t.Fatalf("got %d channels", len(nets[0].Channels))
	}
	if nets[0].Server != "irc.torrentleech.org" {
		t.Errorf("server = %q", nets[0].Server)
	}
}

// TestAuthHeaderIsSent checks the token reaches the wire.
func TestAuthHeaderIsSent(t *testing.T) {
	var got string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-API-Token")
		_, _ = w.Write([]byte(`[]`))
	}))
	if _, err := c.Indexers(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got != "test-token" {
		t.Errorf("X-API-Token = %q", got)
	}
}

// TestServerErrorNamesTheOffset so a failed backfill says where it stopped.
func TestServerErrorNamesTheOffset(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("offset") == "0" {
			_ = json.NewEncoder(w).Encode(releasePage{
				Data: []Release{{ID: 1}, {ID: 2}}, Count: 10,
			})
			return
		}
		w.WriteHeader(http.StatusBadGateway)
	}))

	err := c.Releases(context.Background(), 2, func([]Release) bool { return true })
	if err == nil {
		t.Fatal("a 502 mid-walk did not error")
	}
	if want := fmt.Sprintf("offset %d", 2); !contains(err.Error(), want) {
		t.Errorf("error %q does not name the offset", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestReleaseArrayFieldsAreArrays pins the types that are NOT strings.
//
// Five of autobrr's parsed fields are slices, and the trap is that its own
// database stores them JOINED — reading the SQL column list suggests a string,
// and the decode then fails mid-page and takes the whole release history with
// it. That is exactly what happened: `edition` was typed as a string and the
// autobrr pass aborted at offset 0 with "cannot unmarshal array into Go struct
// field Release.data.edition of type string".
func TestReleaseArrayFieldsAreArrays(t *testing.T) {
	const body = `{
	  "id": 1,
	  "name": "Some.Film.2026.EXTENDED.UNRATED.1080p.BluRay.DV.HDR10.x265-GRP",
	  "codec": ["H.265", "x265"],
	  "hdr": ["DV", "HDR10"],
	  "edition": ["EXTENDED", "UNRATED"],
	  "cut": ["Directors Cut"],
	  "rejections": ["already grabbed"]
	}`
	var r Release
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("a real release did not decode: %v", err)
	}
	if len(r.Edition) != 2 || r.Edition[0] != "EXTENDED" {
		t.Errorf("edition = %v", r.Edition)
	}
	if len(r.Cut) != 1 {
		t.Errorf("cut = %v", r.Cut)
	}
	if len(r.Codec) != 2 || len(r.HDR) != 2 {
		t.Errorf("codec=%v hdr=%v", r.Codec, r.HDR)
	}
	if len(r.Rejections) != 1 {
		t.Errorf("rejections = %v", r.Rejections)
	}
}

// TestReleaseArrayFieldsAcceptNull covers the common case.
//
// Most releases parse none of these, and autobrr sends null rather than an
// empty array. A nil slice is the right decode; it must not error.
func TestReleaseArrayFieldsAcceptNull(t *testing.T) {
	const body = `{"id":1,"name":"x","codec":null,"hdr":null,"edition":null,"cut":null}`
	var r Release
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string][]string{
		"codec": r.Codec, "hdr": r.HDR, "edition": r.Edition, "cut": r.Cut,
	} {
		if len(v) != 0 {
			t.Errorf("%s = %v, want empty", name, v)
		}
	}
}
