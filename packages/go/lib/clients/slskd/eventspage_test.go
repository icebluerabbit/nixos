package slskd_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"homelab/clients/slskd"
	"homelab/httpx"
)

// descendingEvents serves n events NEWEST first with real offset/limit
// semantics, which is what slskd does: offset 0 is today.
func descendingEvents(t *testing.T, n int, queries *[]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	t.Cleanup(srv.Close)
	return srv
}

func pageClient(t *testing.T, srv *httptest.Server) *slskd.Client {
	t.Helper()
	c, err := slskd.New(httpx.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEventsPageAt(t *testing.T) {
	c := pageClient(t, descendingEvents(t, 25, nil))

	got, err := c.EventsPageAt(context.Background(), 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 10 {
		t.Fatalf("got %d events, want 10", len(got))
	}
	if got[0].ID != "evt-10" {
		t.Errorf("first event = %q, want evt-10", got[0].ID)
	}
	if got[0].Type != "DownloadFileComplete" {
		t.Errorf("type = %q", got[0].Type)
	}
}

func TestEventsPageAtShortFinalPage(t *testing.T) {
	c := pageClient(t, descendingEvents(t, 25, nil))

	got, err := c.EventsPageAt(context.Background(), 20, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Errorf("got %d events, want 5", len(got))
	}
}

func TestEventsPageAtPastTheEnd(t *testing.T) {
	c := pageClient(t, descendingEvents(t, 5, nil))

	got, err := c.EventsPageAt(context.Background(), 99, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %d events past the end, want 0", len(got))
	}
}

// A negative offset or a zero limit would ask slskd for something undefined.
func TestEventsPageAtClampsArguments(t *testing.T) {
	var queries []string
	c := pageClient(t, descendingEvents(t, 3, &queries))

	if _, err := c.EventsPageAt(context.Background(), -5, 0); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(queries[0], "offset=-") || strings.Contains(queries[0], "limit=0") {
		t.Errorf("sent an unclamped argument: %q", queries[0])
	}
}

func TestEventsPageAtError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	if _, err := pageClient(t, srv).EventsPageAt(context.Background(), 0, 10); err == nil {
		t.Fatal("expected an error")
	}
}
