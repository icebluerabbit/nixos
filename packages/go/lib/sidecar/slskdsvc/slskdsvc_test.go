package slskdsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"homelab/clients/slskd"
	"homelab/httpx"
	"homelab/sidecar"
)

// newService points a service at a handler standing in for slskd.
func newService(t *testing.T, h http.HandlerFunc) *Service {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := slskd.New(httpx.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return New(c, srv.URL, 500)
}

// eventLog serves n events NEWEST first with real offset/limit semantics, which
// is what slskd does: offset 0 is today.
func eventLog(t *testing.T, n int, queries *[]string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "searches") {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "s1", "searchText": "artist album"},
			})
			return
		}
		if queries != nil {
			*queries = append(*queries, r.URL.RawQuery)
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

		events := []map[string]any{}
		for i := offset; i < offset+limit && i < n; i++ {
			events = append(events, map[string]any{
				"id":        fmt.Sprintf("evt-%d", i),
				"timestamp": "2026-09-14T16:29:40Z",
				"type":      "DownloadFileComplete",
				"data":      `{"localFilename":"/mnt/media/x.flac"}`,
			})
		}
		_ = json.NewEncoder(w).Encode(events)
	}
}

func TestInfo(t *testing.T) {
	got := newService(t, eventLog(t, 0, nil)).Info()

	if got.Sidecar != "managarr-slskd" || got.Source != Source {
		t.Errorf("info = %+v", got)
	}
	// Changes only: slskd has no webhook. Its journal reaches managarr
	// through the Vector sidecar instead, which is a different transport for
	// a different thing — log lines, not application events.
	if len(got.Capabilities) != 1 || got.Capabilities[0] != sidecar.CapChanges {
		t.Errorf("capabilities = %v, want [changes]", got.Capabilities)
	}
}

func TestEnrichUnsupported(t *testing.T) {
	_, err := newService(t, eventLog(t, 1, nil)).
		Enrich(context.Background(), []byte(`{}`))

	if !errors.Is(err, sidecar.ErrUnsupported) {
		t.Errorf("err = %v, want ErrUnsupported", err)
	}
}

func TestChanges(t *testing.T) {
	events, err := newService(t, eventLog(t, 3, nil)).
		Changes(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}

	e := events[0]
	if e.Source != Source || e.Kind != KindEvent {
		t.Errorf("event = %s/%s", e.Source, e.Kind)
	}
	// slskd's own event id, not something dug out of the payload: the
	// transfer events carry the peer and path inside Data, and reaching in
	// here would make this sidecar know one shape out of a dozen.
	if e.Ref != "evt-0" {
		t.Errorf("ref = %q", e.Ref)
	}
	if e.Cursor != "1" {
		t.Errorf("cursor = %q, want the count consumed", e.Cursor)
	}
	if e.OccurredAt.IsZero() {
		t.Error("timestamp was lost")
	}

	got, err := sidecar.DecodePayload[slskd.Event](e)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "DownloadFileComplete" {
		t.Errorf("payload = %+v", got)
	}
}

// The cursor is a descending offset and the walk runs in slskd's own direction.
// New events are prepended, so one arriving mid-walk pushes the rest to a
// HIGHER offset — a page is re-read rather than jumped over. A duplicate costs
// an idempotent upsert; a skip would cost a record that exists nowhere else.
func TestChangesWalksDescending(t *testing.T) {
	var queries []string
	svc := newService(t, eventLog(t, 25, &queries))
	ctx := context.Background()

	first, err := svc.Changes(ctx, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(queries[0], "offset=0") {
		t.Errorf("first query = %q, want offset=0", queries[0])
	}

	second, err := svc.Changes(ctx, first[9].Cursor, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(queries[1], "offset=10") {
		t.Errorf("second query = %q, want offset=10", queries[1])
	}
	if second[0].Ref != "evt-10" {
		t.Errorf("resumed at %q, want evt-10", second[0].Ref)
	}
}

func TestChangesEndOfLog(t *testing.T) {
	svc := newService(t, eventLog(t, 5, nil))

	events, err := svc.Changes(context.Background(), "5", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("got %d events past the end, want 0", len(events))
	}
}

func TestChangesBadCursor(t *testing.T) {
	svc := newService(t, eventLog(t, 1, nil))

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

func TestSearchesRoute(t *testing.T) {
	svc := newService(t, eventLog(t, 0, nil))
	mux := http.NewServeMux()
	svc.Routes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, SearchesPath, http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}

	var body struct {
		Searches []slskd.Search `json:"searches"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Searches) != 1 {
		t.Errorf("searches = %+v", body.Searches)
	}
}

func TestSearchesRouteReportsFailure(t *testing.T) {
	svc := newService(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux := http.NewServeMux()
	svc.Routes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, SearchesPath, http.NoBody))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// A zero page size must fall back to a default rather than requesting nothing
// forever.
func TestNewClampsPageSize(t *testing.T) {
	srv := httptest.NewServer(eventLog(t, 1, nil))
	t.Cleanup(srv.Close)

	c, err := slskd.New(httpx.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if got := New(c, srv.URL, 0).pageSize; got <= 0 {
		t.Errorf("pageSize = %d, want a positive default", got)
	}
}

func TestImplementsContract(t *testing.T) {
	var (
		_ sidecar.Service  = (*Service)(nil)
		_ sidecar.Extender = (*Service)(nil)
	)
}

// Every download event embeds its whole transfer object, so a page the contract
// would allow — 5000 — is hundreds of megabytes. The sidecar caps below what
// the consumer asks for rather than trusting it.
func TestChangesCapsPageSize(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(eventLog(t, 10, &queries))
	t.Cleanup(srv.Close)

	c, err := slskd.New(httpx.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	svc := New(c, srv.URL, 5)

	if _, err := svc.Changes(context.Background(), "", 5000); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(queries[0], "limit=5") {
		t.Errorf("query = %q, want the cap applied", queries[0])
	}
}
