package prowlarr_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"homelab/clients/prowlarr"
	"homelab/httpx"
)

// ascendingHistory serves n records of whichever event type is asked for, with
// real page/pageSize semantics and every query recorded.
func ascendingHistory(t *testing.T, n int, queries *[]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if queries != nil {
			*queries = append(*queries, r.URL.RawQuery)
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		size, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
		if page < 1 {
			page = 1
		}

		records := []map[string]any{}
		for i := (page - 1) * size; i < page*size && i < n; i++ {
			records = append(records, map[string]any{
				"id": i, "indexerId": 1,
				"date":      "2026-01-01T00:00:00Z",
				"eventType": "releaseGrabbed",
				"data":      map[string]any{"title": fmt.Sprintf("Release.%d", i)},
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": page, "pageSize": size,
			"totalRecords": n, "records": records,
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func pageClient(t *testing.T, srv *httptest.Server) *prowlarr.Client {
	t.Helper()
	c, err := prowlarr.New(httpx.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// The opposite ordering to History, deliberately. History is newest-first
// because its caller stops as soon as it recognises a record; this is for a
// cursor, and only ascending gives a stable offset.
func TestHistoryPageAtSortsAscending(t *testing.T) {
	var queries []string
	c := pageClient(t, ascendingHistory(t, 3, &queries))

	_, err := c.HistoryPageAt(context.Background(), prowlarr.EventReleaseGrabbed, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(queries[0], "sortDirection=ascending") {
		t.Errorf("query = %q, want ascending", queries[0])
	}
}

// The event type has to reach Prowlarr as its number: the API filters on the
// numeric form, and sending the name returns everything.
func TestHistoryPageAtSendsTheEventType(t *testing.T) {
	var queries []string
	c := pageClient(t, ascendingHistory(t, 1, &queries))

	for _, event := range []prowlarr.EventType{
		prowlarr.EventReleaseGrabbed,
		prowlarr.EventIndexerQuery,
		prowlarr.EventIndexerRss,
	} {
		if _, err := c.HistoryPageAt(context.Background(), event, 1, 10); err != nil {
			t.Fatal(err)
		}
	}
	for i, want := range []string{"eventType=1", "eventType=2", "eventType=3"} {
		if !strings.Contains(queries[i], want) {
			t.Errorf("query %d = %q, want %s", i, queries[i], want)
		}
	}
}

func TestHistoryPageAt(t *testing.T) {
	c := pageClient(t, ascendingHistory(t, 25, nil))

	got, err := c.HistoryPageAt(context.Background(), prowlarr.EventIndexerQuery, 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 10 {
		t.Fatalf("got %d records, want 10", len(got.Records))
	}
	// The total is what tells a cursor it has reached the end of a stage.
	if got.TotalRecords != 25 {
		t.Errorf("total = %d, want 25", got.TotalRecords)
	}
	if got.Records[0].Str("title") != "Release.10" {
		t.Errorf("first record = %q", got.Records[0].Str("title"))
	}
}

func TestHistoryPageAtShortFinalPage(t *testing.T) {
	c := pageClient(t, ascendingHistory(t, 25, nil))

	got, err := c.HistoryPageAt(context.Background(), prowlarr.EventIndexerRss, 3, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 5 {
		t.Errorf("got %d records, want 5", len(got.Records))
	}
}

func TestHistoryPageAtClampsArguments(t *testing.T) {
	var queries []string
	c := pageClient(t, ascendingHistory(t, 3, &queries))

	_, err := c.HistoryPageAt(context.Background(), prowlarr.EventReleaseGrabbed, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(queries[0], "page=1") {
		t.Errorf("query = %q, want page=1", queries[0])
	}
	if strings.Contains(queries[0], "pageSize=0") {
		t.Errorf("query = %q, want a non-zero page size", queries[0])
	}
}

func TestHistoryPageAtError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	_, err := pageClient(t, srv).HistoryPageAt(
		context.Background(), prowlarr.EventReleaseGrabbed, 1, 10)
	if err == nil {
		t.Fatal("expected an error")
	}
}
