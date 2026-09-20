package qbittorrent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"homelab/httpx"
)

func newTestClient(t *testing.T, h http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := New(httpx.Config{BaseURL: srv.URL, RetryMax: -1})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestTorrentsDecodesBothInfohashes is the join key's subtlety.
//
// `hash` is the client's id for the torrent. For a v1 torrent it equals
// infohash_v1; for a v2 or hybrid one it does NOT. The *arr apps report the v1
// hash as downloadId, so keying on `hash` alone would silently fail to join
// every v2 torrent to its grab.
func TestTorrentsDecodesBothInfohashes(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{
		  "hash": "aaaa0000000000000000000000000000aaaa0000",
		  "infohash_v1": "bbbb0000000000000000000000000000bbbb0000",
		  "infohash_v2": "cccc000000000000000000000000000000000000000000000000000000000000",
		  "name": "Some.Release-GRP",
		  "category": "tv",
		  "tags": "cross-seed, manual",
		  "state": "stalledUP",
		  "private": true,
		  "save_path": "/mnt/media/complete/tv",
		  "content_path": "/mnt/media/complete/tv/Some.Release-GRP",
		  "total_size": 123456789,
		  "added_on": 1757800000,
		  "completion_on": 1757800600,
		  "tracker": "https://tracker.example/announce/deadbeef"
		}]`))
	}))

	got, err := c.Torrents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d torrents", len(got))
	}
	tor := got[0]
	if tor.Hash == tor.InfoHashV1 {
		t.Fatal("fixture is wrong: the two hashes should differ here")
	}
	if tor.InfoHashV1 == "" || tor.InfoHashV2 == "" {
		t.Errorf("infohashes = %q / %q", tor.InfoHashV1, tor.InfoHashV2)
	}
	// Private decides whether hit-and-run rules apply at all, and is null
	// before metadata arrives — hence the pointer.
	if tor.Private == nil || !*tor.Private {
		t.Errorf("private = %v", tor.Private)
	}
	if tor.State != StateStalledUP {
		t.Errorf("state = %q", tor.State)
	}
	if tor.CompletionOn != 1757800600 {
		t.Errorf("completion_on = %d", tor.CompletionOn)
	}
}

// TestTorrentsPrivateIsNullBeforeMetadata covers the pointer's reason.
//
// A magnet that has not resolved reports no `private` key. Decoding it as a
// bare bool would make every such torrent read as public, and public is the
// case where hit-and-run rules do not apply.
func TestTorrentsPrivateIsNullBeforeMetadata(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"hash":"a","name":"x","state":"metaDL"}]`))
	}))
	got, err := c.Torrents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Private != nil {
		t.Errorf("private = %v, want nil", *got[0].Private)
	}
}

// TestFilesSendsTheHash checks the per-torrent query.
func TestFilesSendsTheHash(t *testing.T) {
	var q url.Values
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		_, _ = w.Write([]byte(`[
		  {"name":"Some.Release-GRP/file.mkv","size":123,"progress":1},
		  {"name":"Some.Release-GRP/sample.rar","size":45,"progress":1}
		]`))
	}))

	got, err := c.Files(context.Background(), "AAAA")
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("hash") != "AAAA" {
		t.Errorf("hash = %q", q.Get("hash"))
	}
	if len(got) != 2 {
		t.Fatalf("got %d files", len(got))
	}
	if got[0].Name == "" || got[0].Size == 0 {
		t.Errorf("file = %+v", got[0])
	}
}

// TestTrackersDecodesStatusAndMessage is where unregistered torrents are
// detected.
//
// The message is the tracker's own words — "Unregistered torrent", "Torrent not
// found" — and is the only evidence a torrent has been pulled. The status code
// alone does not distinguish "not working yet" from "never again".
func TestTrackersDecodesStatusAndMessage(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
		  {"url":"** [DHT] **","status":0,"msg":""},
		  {"url":"https://tracker.example/announce/deadbeef","status":4,"msg":"Unregistered torrent"}
		]`))
	}))

	got, err := c.Trackers(context.Background(), "AAAA")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d trackers", len(got))
	}
	announce := got[1]
	if announce.Status != TrackerNotWorking {
		t.Errorf("status = %d, want %d", announce.Status, TrackerNotWorking)
	}
	if !IsUnregistered(announce.Msg) {
		t.Errorf("message %q was not recognised as unregistered", announce.Msg)
	}
	// The pseudo-entries qBittorrent injects must survive rather than being
	// filtered here: what counts as a real tracker is the caller's policy.
	if got[0].URL != "** [DHT] **" {
		t.Errorf("pseudo tracker = %q", got[0].URL)
	}
}

// TestServerErrorIsAnError keeps a down client from reading as an empty one.
//
// This matters more here than anywhere else: the reconcile marks torrents
// removed when they stop appearing, so an empty list mistaken for the truth
// would mark the entire client's contents as gone.
func TestServerErrorIsAnError(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	if _, err := c.Torrents(context.Background()); err == nil {
		t.Fatal("a 403 did not produce an error")
	}
}

func TestMalformedJSONIsAnError(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"hash":`))
	}))
	if _, err := c.Torrents(context.Background()); err == nil {
		t.Fatal("truncated JSON did not produce an error")
	}
}

// TestTorrentsDecodesProgress covers the one point-in-time field this client
// keeps.
//
// State does not carry it: stalledDL describes a torrent stuck at 3% and one
// stuck at 99% identically, so without progress a partial download is
// indistinguishable from a complete one that is merely idle.
func TestTorrentsDecodesProgress(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
		  {"hash":"a","name":"partial","state":"stalledDL","progress":0.0312},
		  {"hash":"b","name":"done","state":"stalledUP","progress":1},
		  {"hash":"c","name":"nothing","state":"metaDL"}
		]`))
	}))

	got, err := c.Torrents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d torrents", len(got))
	}
	if got[0].Progress != 0.0312 {
		t.Errorf("partial progress = %v", got[0].Progress)
	}
	// A complete torrent is exactly 1, not 100.
	if got[1].Progress != 1 {
		t.Errorf("complete progress = %v, want 1", got[1].Progress)
	}
	// Absent before metadata arrives, and zero is the honest decode of that.
	if got[2].Progress != 0 {
		t.Errorf("metadata-less progress = %v", got[2].Progress)
	}
}
