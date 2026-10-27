package qbittorrentsvc

import (
	"context"
	"encoding/json"

	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"homelab/clients/qbittorrent"
	"homelab/httpx"
	"homelab/sidecar"
)

// newService points a service at a handler standing in for qBittorrent.
func newService(t *testing.T, h http.HandlerFunc) *Service {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := qbittorrent.New(httpx.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return New(c, srv.URL)
}

// torrent builds one entry of qBittorrent's list.
func torrent(hash string, addedOn int64) map[string]any {
	return map[string]any{
		"hash": hash, "name": "Name." + hash, "added_on": addedOn,
		"completion_on": addedOn + 60, "state": "uploading",
		"save_path": "/mnt/media/complete", "total_size": 100,
	}
}

// listing serves a fixed torrent list on /torrents/info and nothing else.
func listing(t *testing.T, torrents ...map[string]any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "torrents/info"):
			_ = json.NewEncoder(w).Encode(torrents)
		case strings.Contains(r.URL.Path, "torrents/files"):
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"name": "a.mkv", "size": 100, "index": 0},
			})
		case strings.Contains(r.URL.Path, "torrents/trackers"):
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"url": "https://tracker.example/announce", "status": 2},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestInfo(t *testing.T) {
	got := newService(t, listing(t)).Info()

	if got.Sidecar != "managarr-qbittorrent" || got.Source != Source {
		t.Errorf("info = %+v", got)
	}
	if len(got.Capabilities) != 2 {
		t.Errorf("capabilities = %v", got.Capabilities)
	}
}

func TestChanges(t *testing.T) {
	svc := newService(t, listing(t,
		torrent("bbb", 200), torrent("aaa", 100), torrent("ccc", 300)))

	events, err := svc.Changes(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}

	// Oldest first, whatever order qBittorrent listed them in.
	for i, want := range []string{"aaa", "bbb", "ccc"} {
		if events[i].Ref != want {
			t.Errorf("event %d ref = %q, want %q", i, events[i].Ref, want)
		}
	}
	if events[0].Kind != KindAdded {
		t.Errorf("kind = %q", events[0].Kind)
	}
	if events[0].Cursor != "100:aaa" {
		t.Errorf("cursor = %q", events[0].Cursor)
	}
}

func TestChangesResumes(t *testing.T) {
	svc := newService(t, listing(t,
		torrent("aaa", 100), torrent("bbb", 200), torrent("ccc", 300)))

	events, err := svc.Changes(context.Background(), "200:bbb", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Ref != "ccc" {
		t.Errorf("events = %+v, want only ccc", events)
	}
}

// The whole reason the cursor carries a hash. qBittorrent records added_on in
// whole seconds, and a *arr grabbing a season pack adds several torrents inside
// one — a timestamp-only cursor would re-read that group forever or step past
// part of it.
func TestChangesTiedTimestamps(t *testing.T) {
	svc := newService(t, listing(t,
		torrent("aaa", 100), torrent("bbb", 100), torrent("ccc", 100)))
	ctx := context.Background()

	seen := map[string]int{}
	cursor := ""
	for range 5 {
		events, err := svc.Changes(ctx, cursor, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) == 0 {
			break
		}
		for _, e := range events {
			seen[e.Ref]++
		}
		cursor = events[len(events)-1].Cursor
	}

	if len(seen) != 3 {
		t.Errorf("saw %d distinct torrents, want 3: %v", len(seen), seen)
	}
	for hash, n := range seen {
		if n != 1 {
			t.Errorf("%s appeared %d times, want once", hash, n)
		}
	}
}

func TestChangesRespectsLimit(t *testing.T) {
	svc := newService(t, listing(t,
		torrent("aaa", 100), torrent("bbb", 200), torrent("ccc", 300)))

	events, err := svc.Changes(context.Background(), "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Errorf("got %d events, want 2", len(events))
	}
}

func TestChangesBadCursor(t *testing.T) {
	svc := newService(t, listing(t))

	for _, c := range []string{"nocolon", "abc:aaa"} {
		if _, err := svc.Changes(context.Background(), c, 10); err == nil {
			t.Errorf("cursor %q was accepted", c)
		}
	}
}

func TestChangesUpstreamError(t *testing.T) {
	svc := newService(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	if _, err := svc.Changes(context.Background(), "", 10); err == nil {
		t.Fatal("expected an error")
	}
}

// The point of this sidecar. qBittorrent's hook passes a handful of macros and
// nothing else; the file list and trackers are reachable only while the torrent
// is still in the client, which after an import is minutes.
func TestEnrichFillsInTheCompletion(t *testing.T) {
	svc := newService(t, listing(t, torrent("aaa", 100)))

	events, err := svc.Enrich(context.Background(),
		[]byte(`{"infohash":"AAA","name":"Name.aaa","category":"tv","save_path":"/x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Kind != KindCompleted {
		t.Errorf("kind = %q", events[0].Kind)
	}
	// Lowercased: the hook reports the hash uppercase and the rest of the
	// stack keys on lowercase.
	if events[0].Ref != "aaa" {
		t.Errorf("ref = %q, want the lowercased hash", events[0].Ref)
	}

	var got enriched
	if err := json.Unmarshal(events[0].Payload, &got); err != nil {
		t.Fatal(err)
	}
	if got.Torrent == nil {
		t.Error("torrent was not attached")
	}
	if len(got.Files) != 1 {
		t.Errorf("files = %+v", got.Files)
	}
	if len(got.Trackers) != 1 {
		t.Errorf("trackers = %+v", got.Trackers)
	}
	if got.Category != "tv" {
		t.Errorf("completion was not preserved: %+v", got.completion)
	}
}

// A lookup that fails must not drop the event. Once the torrent is gone nothing
// else in the stack will ever mention it, so the bare completion is worth more
// than nothing.
func TestEnrichSurvivesLookupFailure(t *testing.T) {
	svc := newService(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	events, err := svc.Enrich(context.Background(), []byte(`{"infohash":"aaa","name":"n"}`))
	if err != nil {
		t.Fatalf("lookup failures must not fail the event: %v", err)
	}
	var got enriched
	if err := json.Unmarshal(events[0].Payload, &got); err != nil {
		t.Fatal(err)
	}
	if got.Torrent != nil || got.Files != nil || got.Trackers != nil {
		t.Error("expected the bare completion")
	}
	if got.Infohash != "aaa" {
		t.Error("the completion itself was lost")
	}
}

// A completion with no hash identifies nothing and cannot be correlated, which
// makes it worse than useless — it would land in managarr as a row about
// nothing.
func TestEnrichRejectsBadBodies(t *testing.T) {
	svc := newService(t, listing(t))

	for name, body := range map[string]string{
		"not json": "nonsense",
		"no hash":  `{"name":"n"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.Enrich(context.Background(), []byte(body)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestRoutes(t *testing.T) {
	svc := newService(t, listing(t, torrent("aaa", 100)))
	mux := http.NewServeMux()
	svc.Routes(mux)

	for _, c := range []struct {
		path string
		key  string
	}{
		{TorrentsPath, "torrents"},
		{FilesPath + "?hash=aaa", "files"},
		{TrackersPath + "?hash=aaa", "trackers"},
	} {
		t.Run(c.key, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, http.NoBody))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body)
			}
			var body map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if _, ok := body[c.key]; !ok {
				t.Errorf("response has no %q: %s", c.key, rec.Body)
			}
		})
	}
}

// A per-torrent route with no hash is a caller bug, not a server error.
func TestRoutesRequireHash(t *testing.T) {
	svc := newService(t, listing(t))
	mux := http.NewServeMux()
	svc.Routes(mux)

	for _, path := range []string{FilesPath, TrackersPath} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", path, rec.Code)
		}
	}
}

func TestRoutesReportUpstreamFailure(t *testing.T) {
	svc := newService(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux := http.NewServeMux()
	svc.Routes(mux)

	for _, path := range []string{TorrentsPath, FilesPath + "?hash=a", TrackersPath + "?hash=a"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("%s = %d, want 500", path, rec.Code)
		}
	}
}

// Empty must be an array, never null: managarr ranges over these, and null
// reads the same as a failed request.
func TestRoutesEmptyIsAnArray(t *testing.T) {
	svc := newService(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	})
	mux := http.NewServeMux()
	svc.Routes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, TorrentsPath, http.NoBody))

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if string(body["torrents"]) != "[]" {
		t.Errorf("torrents = %s, want []", body["torrents"])
	}
}

// qBittorrent reports 0 when it has nothing to say, which is not an instant at
// the epoch — recording it as one would date every such torrent to 1970.
func TestUnixZeroIsNoTime(t *testing.T) {
	for _, secs := range []int64{0, -1} {
		if got := unix(secs); !got.IsZero() {
			t.Errorf("unix(%d) = %v, want the zero time", secs, got)
		}
	}
	if got := unix(1700000000); got.IsZero() {
		t.Error("a real timestamp became the zero time")
	}
}

func TestImplementsContract(t *testing.T) {
	var (
		_ sidecar.Service  = (*Service)(nil)
		_ sidecar.Extender = (*Service)(nil)
	)

}
