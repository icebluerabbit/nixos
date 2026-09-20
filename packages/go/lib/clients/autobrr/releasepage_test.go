package autobrr_test

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
)

// descendingReleases serves n releases NEWEST first, which is how autobrr
// serves them, with real limit/offset semantics. The reversal is what is under
// test, so a handler returning them ascending would prove nothing.
func descendingReleases(t *testing.T, n int, queries *[]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if queries != nil {
			*queries = append(*queries, r.URL.RawQuery)
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		data := []map[string]any{}
		for i := offset; i < offset+limit && i < n; i++ {
			// Index 0 is the newest, which is release n-1.
			id := n - 1 - i
			data = append(data, map[string]any{
				"id": id, "name": fmt.Sprintf("Release.%d", id),
				"timestamp": "2026-01-01T00:00:00Z",
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "count": n})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func pageClient(t *testing.T, srv *httptest.Server) *autobrr.Client {
	t.Helper()
	c, err := autobrr.New(httpx.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// The whole reason this method exists. autobrr serves newest-first and pages by
// offset, so a descending offset cannot be a cursor: a release arriving
// mid-walk shifts every later page down by one and a row is skipped. Counting
// from the oldest end gives a position new releases cannot move.
func TestReleasesPageAscReturnsOldestFirst(t *testing.T) {
	c := pageClient(t, descendingReleases(t, 5, nil))

	got, err := c.ReleasesPageAsc(context.Background(), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 5 {
		t.Fatalf("got %d records, want 5", len(got.Records))
	}
	for i := range 5 {
		want := fmt.Sprintf("Release.%d", i)
		if got.Records[i].Name != want {
			t.Errorf("record %d = %q, want %q", i, got.Records[i].Name, want)
		}
	}
	if got.Total != 5 {
		t.Errorf("total = %d, want 5", got.Total)
	}
}

func TestReleasesPageAscPages(t *testing.T) {
	c := pageClient(t, descendingReleases(t, 25, nil))
	ctx := context.Background()

	first, err := c.ReleasesPageAsc(ctx, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.ReleasesPageAsc(ctx, 10, 10)
	if err != nil {
		t.Fatal(err)
	}

	if first.Records[0].Name != "Release.0" {
		t.Errorf("first page starts at %q", first.Records[0].Name)
	}
	if second.Records[0].Name != "Release.10" {
		t.Errorf("second page starts at %q", second.Records[0].Name)
	}
	if len(second.Records) != 10 {
		t.Errorf("second page = %d records, want 10", len(second.Records))
	}
}

// The oldest page is where the descending offset would go negative. Clamping
// has to shorten the request rather than send autobrr a negative offset.
func TestReleasesPageAscClampsAtTheOldestEnd(t *testing.T) {
	var queries []string
	c := pageClient(t, descendingReleases(t, 5, &queries))

	got, err := c.ReleasesPageAsc(context.Background(), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 5 {
		t.Errorf("got %d records, want all 5", len(got.Records))
	}
	for _, q := range queries {
		if strings.Contains(q, "offset=-") {
			t.Errorf("sent a negative offset: %q", q)
		}
	}
}

func TestReleasesPageAscPastTheEnd(t *testing.T) {
	c := pageClient(t, descendingReleases(t, 5, nil))

	got, err := c.ReleasesPageAsc(context.Background(), 99, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 0 {
		t.Errorf("got %d records past the end, want 0", len(got.Records))
	}
	// The total still has to come back, or a caller cannot tell "past the
	// end" from "autobrr is empty".
	if got.Total != 5 {
		t.Errorf("total = %d, want 5", got.Total)
	}
}

func TestReleasesPageAscEmptyCorpus(t *testing.T) {
	c := pageClient(t, descendingReleases(t, 0, nil))

	got, err := c.ReleasesPageAsc(context.Background(), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 0 || got.Total != 0 {
		t.Errorf("page = %+v", got)
	}
}

func TestReleasesPageAscClampsArguments(t *testing.T) {
	var queries []string
	c := pageClient(t, descendingReleases(t, 3, &queries))

	if _, err := c.ReleasesPageAsc(context.Background(), -5, 0); err != nil {
		t.Fatal(err)
	}
	for _, q := range queries {
		if strings.Contains(q, "offset=-") || strings.Contains(q, "limit=0") {
			t.Errorf("sent an unclamped argument: %q", q)
		}
	}
}

func TestReleasesPageAscError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	if _, err := pageClient(t, srv).ReleasesPageAsc(context.Background(), 0, 10); err == nil {
		t.Fatal("expected an error")
	}
}
