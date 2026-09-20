package arr_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"homelab/clients/arr"
	"homelab/httpx"
)

// pagedHistory serves n records with real page/pageSize semantics and records
// every query, so the ordering this method depends on is asserted rather than
// assumed.
func pagedHistory(t *testing.T, n int, queries *[]string) *httptest.Server {
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
				"id": i, "eventType": "grabbed",
				"sourceTitle": fmt.Sprintf("Release.%d", i),
				"downloadId":  fmt.Sprintf("%040X", i),
				"date":        "2026-01-01T00:00:00Z",
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

func pagedClient(t *testing.T, srv *httptest.Server) *arr.Client {
	t.Helper()
	c, err := arr.New(arr.Sonarr, httpx.Config{BaseURL: srv.URL}, "key")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// Ascending is the whole reason this method exists separately from History.
// History walks newest-first for a caller that stops when it recognises a
// record; an offset cursor is only safe on an ascending order, where a record
// arriving mid-walk lands at the end and cannot shift a page already read.
func TestHistoryPageAtSortsAscending(t *testing.T) {
	var queries []string
	c := pagedClient(t, pagedHistory(t, 3, &queries))

	if _, err := c.HistoryPageAt(context.Background(), 1, 10); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(queries[0], "sortDirection=ascending") {
		t.Errorf("query = %q, want ascending", queries[0])
	}
	if !strings.Contains(queries[0], "sortKey=date") {
		t.Errorf("query = %q, want sortKey=date", queries[0])
	}
}

func TestHistoryPageAt(t *testing.T) {
	c := pagedClient(t, pagedHistory(t, 25, nil))

	got, err := c.HistoryPageAt(context.Background(), 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 10 {
		t.Fatalf("got %d records, want 10", len(got.Records))
	}
	// The total is what tells a cursor it has reached the end, so it has to
	// survive rather than be dropped with the rest of the envelope.
	if got.TotalRecords != 25 {
		t.Errorf("total = %d, want 25", got.TotalRecords)
	}
	if got.Records[0].SourceTitle != "Release.10" {
		t.Errorf("first record = %q", got.Records[0].SourceTitle)
	}
}

func TestHistoryPageAtShortFinalPage(t *testing.T) {
	c := pagedClient(t, pagedHistory(t, 25, nil))

	got, err := c.HistoryPageAt(context.Background(), 3, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 5 {
		t.Errorf("got %d records, want 5", len(got.Records))
	}
}

func TestHistoryPageAtPastTheEnd(t *testing.T) {
	c := pagedClient(t, pagedHistory(t, 5, nil))

	got, err := c.HistoryPageAt(context.Background(), 99, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 0 {
		t.Errorf("got %d records past the end, want 0", len(got.Records))
	}
}

// A zero or negative page must not be sent to the *arr as-is: page 0 is not a
// thing, and the response to it is not defined.
func TestHistoryPageAtClampsArguments(t *testing.T) {
	var queries []string
	c := pagedClient(t, pagedHistory(t, 3, &queries))

	if _, err := c.HistoryPageAt(context.Background(), 0, 0); err != nil {
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

	if _, err := pagedClient(t, srv).HistoryPageAt(context.Background(), 1, 10); err == nil {
		t.Fatal("expected an error")
	}
}
