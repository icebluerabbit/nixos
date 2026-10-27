package autobrrsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"homelab/clients/autobrr"
	"homelab/httpx"
	"homelab/sidecar"
)

// newService points a service at a handler standing in for autobrr.
func newService(t *testing.T, h http.HandlerFunc) *Service {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := autobrr.New(httpx.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return New(c, srv.URL)
}

// releases serves n releases NEWEST first, which is how autobrr serves them,
// with real limit/offset semantics. The reversal is the thing under test, so a
// handler that returned them ascending would prove nothing.
func releases(t *testing.T, n int, queries *[]string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "release"):
			if queries != nil {
				*queries = append(*queries, r.URL.RawQuery)
			}
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

			data := []map[string]any{}
			for i := offset; i < offset+limit && i < n; i++ {
				// Descending: index 0 is the newest, which is release n-1.
				id := n - 1 - i
				data = append(data, map[string]any{
					"id": id, "name": fmt.Sprintf("Release.%d", id),
					"filter_status": "FILTER_APPROVED",
					"timestamp":     "2026-01-01T00:00:00Z",
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": data, "count": n, "next_cursor": 0,
			})
		case strings.Contains(r.URL.Path, "indexer"):
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": 1, "name": "tracker"}})
		case strings.Contains(r.URL.Path, "filter"):
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": 1, "name": "tv"}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestInfo(t *testing.T) {
	got := newService(t, releases(t, 0, nil)).Info()

	if got.Sidecar != "managarr-autobrr" || got.Source != Source {
		t.Errorf("info = %+v", got)
	}
	if len(got.Capabilities) != 2 {
		t.Errorf("capabilities = %v", got.Capabilities)
	}
}

// Ascending is the whole point. autobrr serves newest-first and pages by
// offset, which cannot be a cursor: a release arriving mid-walk shifts every
// later page down by one and a row is skipped. Counting from the oldest end
// gives a position new releases cannot move.
func TestChangesReturnsOldestFirst(t *testing.T) {
	events, err := newService(t, releases(t, 5, nil)).
		Changes(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 5 {
		t.Fatalf("got %d events, want 5", len(events))
	}

	for i := range 5 {
		want := fmt.Sprintf("Release.%d", i)
		if events[i].Ref != want {
			t.Errorf("event %d ref = %q, want %q", i, events[i].Ref, want)
		}
	}
	if events[0].Kind != KindRelease {
		t.Errorf("kind = %q", events[0].Kind)
	}
	if events[4].Cursor != "5" {
		t.Errorf("cursor = %q, want 5", events[4].Cursor)
	}
}

func TestChangesResumes(t *testing.T) {
	svc := newService(t, releases(t, 25, nil))

	events, err := svc.Changes(context.Background(), "10", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 10 {
		t.Fatalf("got %d events, want 10", len(events))
	}
	if events[0].Ref != "Release.10" {
		t.Errorf("resumed at %q, want Release.10", events[0].Ref)
	}
}

// The oldest page is where the descending offset would go negative. Clamping it
// has to shorten the request rather than ask autobrr for a negative offset.
func TestChangesAtTheOldestEnd(t *testing.T) {
	var queries []string
	svc := newService(t, releases(t, 5, &queries))

	events, err := svc.Changes(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 5 {
		t.Fatalf("got %d events, want all 5", len(events))
	}
	for _, q := range queries {
		if strings.Contains(q, "offset=-") {
			t.Errorf("asked autobrr for a negative offset: %q", q)
		}
	}
}

func TestChangesPastTheEnd(t *testing.T) {
	events, err := newService(t, releases(t, 5, nil)).
		Changes(context.Background(), "5", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("got %d events past the end, want 0", len(events))
	}
}

func TestChangesEmptyCorpus(t *testing.T) {
	events, err := newService(t, releases(t, 0, nil)).
		Changes(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("got %d events, want 0", len(events))
	}
}

func TestChangesBadCursor(t *testing.T) {
	svc := newService(t, releases(t, 1, nil))

	for _, c := range []string{"abc", "-1"} {
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

// The action body is written by the filter author from autobrr's macros, not by
// autobrr, so the sidecar keeps it whole rather than reaching into it.
func TestEnrichForwardsTheActionBody(t *testing.T) {
	const body = `{"name":"Some.Release-GRP","indexer":"tracker","extra":{"a":1}}`

	events, err := newService(t, releases(t, 0, nil)).
		Enrich(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Kind != KindAction {
		t.Errorf("kind = %q", events[0].Kind)
	}
	if events[0].Ref != "Some.Release-GRP" {
		t.Errorf("ref = %q", events[0].Ref)
	}
	if string(events[0].Payload) != body {
		t.Errorf("payload = %s, want it unchanged", events[0].Payload)
	}
}

// The filters here have used either spelling depending on when they were
// written, and a miss puts nothing in Ref rather than failing loudly.
func TestEnrichAcceptsEitherNameField(t *testing.T) {
	svc := newService(t, releases(t, 0, nil))

	for _, body := range []string{
		`{"name":"R"}`,
		`{"torrent_name":"R"}`,
		`{"release_name":"R"}`,
	} {
		events, err := svc.Enrich(context.Background(), []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		if events[0].Ref != "R" {
			t.Errorf("body %s gave ref %q", body, events[0].Ref)
		}
	}
}

func TestEnrichRejectsNonJSON(t *testing.T) {
	_, err := newService(t, releases(t, 0, nil)).
		Enrich(context.Background(), []byte("nonsense"))

	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestRoutes(t *testing.T) {
	svc := newService(t, releases(t, 0, nil))
	mux := http.NewServeMux()
	svc.Routes(mux)

	for path, key := range map[string]string{
		IndexersPath: "indexers",
		FiltersPath:  "filters",
	} {
		t.Run(key, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body)
			}
			var body map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if _, ok := body[key]; !ok {
				t.Errorf("response has no %q: %s", key, rec.Body)
			}
		})
	}
}

func TestRoutesReportUpstreamFailure(t *testing.T) {
	svc := newService(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux := http.NewServeMux()
	svc.Routes(mux)

	for _, path := range []string{IndexersPath, FiltersPath} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("%s = %d, want 500", path, rec.Code)
		}
	}
}

func TestImplementsContract(t *testing.T) {
	var (
		_ sidecar.Service  = (*Service)(nil)
		_ sidecar.Extender = (*Service)(nil)
	)
}
