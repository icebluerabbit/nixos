package prowlarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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
		Headers:  http.Header{"X-Api-Key": []string{"test-key"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestDataMapFlattensEveryJSONType is the tolerant-decode contract.
//
// Prowlarr's history `data` is typed as a string map but is not one: it carries
// bare numbers, bare booleans, nulls and occasionally nested objects, and the
// mix varies by event type and by version. A strict map[string]string decode
// fails the whole page — losing 72,000 records because one had a numeric
// elapsedTime.
func TestDataMapFlattensEveryJSONType(t *testing.T) {
	const body = `{
	  "string": "text",
	  "number": 1234,
	  "float": 12.5,
	  "bignum": 9007199254740993,
	  "boolTrue": true,
	  "boolFalse": false,
	  "null": null,
	  "object": {"a": 1},
	  "array": [1, 2]
	}`
	var d DataMap
	if err := json.Unmarshal([]byte(body), &d); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"string":    "text",
		"number":    "1234",
		"float":     "12.5",
		"boolTrue":  "true",
		"boolFalse": "false",
		"object":    `{"a":1}`,
		"array":     `[1,2]`,
	}
	for k, v := range want {
		if d[k] != v {
			t.Errorf("data[%q] = %q, want %q", k, d[k], v)
		}
	}
	// A number must not come back in exponent form or with a trailing ".0",
	// because the caller parses it straight back to an integer.
	if d["bignum"] != "9007199254740993" && d["bignum"] != "9007199254740992" {
		t.Errorf("bignum = %q", d["bignum"])
	}
	if _, ok := d["null"]; ok && d["null"] != "" {
		t.Errorf("null became %q", d["null"])
	}
}

func TestDataMapNullIsAnEmptyMap(t *testing.T) {
	var d DataMap
	if err := json.Unmarshal([]byte(`null`), &d); err != nil {
		t.Fatal(err)
	}
	if len(d) != 0 {
		t.Errorf("null data = %v", d)
	}
}

// TestRecordAccessors covers the "present and numeric" distinction.
//
// An absent elapsedTime and an elapsedTime of zero are different facts: one is
// "Prowlarr did not report" and the other is "the query was instant". Both must
// be distinguishable, or every unreported field reads as a measurement of zero.
func TestRecordAccessors(t *testing.T) {
	r := HistoryRecord{Data: DataMap{
		"query":       "some show",
		"elapsedTime": "1234",
		"zero":        "0",
		"empty":       "",
		"notANumber":  "soon",
		"cached":      "1",
		"notCached":   "0",
	}}

	if got := r.Str("query"); got != "some show" {
		t.Errorf("Str = %q", got)
	}
	if got := r.Str("absent"); got != "" {
		t.Errorf("Str(absent) = %q", got)
	}

	if v, ok := r.Int("elapsedTime"); !ok || v != 1234 {
		t.Errorf("Int = %d, %v", v, ok)
	}
	if v, ok := r.Int("zero"); !ok || v != 0 {
		t.Errorf(`Int("0") = %d, %v; a reported zero is present`, v, ok)
	}
	for _, key := range []string{"absent", "empty", "notANumber"} {
		if v, ok := r.Int(key); ok {
			t.Errorf("Int(%q) = %d, true; want not-present", key, v)
		}
	}

	if v, ok := r.Bool("cached"); !ok || !v {
		t.Errorf("Bool(cached) = %v, %v", v, ok)
	}
	if v, ok := r.Bool("notCached"); !ok || v {
		t.Errorf("Bool(notCached) = %v, %v", v, ok)
	}
	if _, ok := r.Bool("absent"); ok {
		t.Error("Bool(absent) reported present")
	}
}

// TestHistorySendsAnExplicitSort is what makes the incremental stop sound.
//
// The caller stops at the first page holding nothing new, which is only
// correct on a newest-first ordering. Relying on the API's default would mean
// an upstream change silently turns every pass into a no-op that still logs
// success.
func TestHistorySendsAnExplicitSort(t *testing.T) {
	var q url.Values
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		_ = json.NewEncoder(w).Encode(historyPage{})
	}))

	if err := c.History(context.Background(), EventIndexerQuery, 100,
		func([]HistoryRecord) bool { return true }); err != nil {
		t.Fatal(err)
	}
	if q.Get("sortKey") != "date" {
		t.Errorf("sortKey = %q", q.Get("sortKey"))
	}
	if q.Get("sortDirection") != "descending" {
		t.Errorf("sortDirection = %q", q.Get("sortDirection"))
	}
	if q.Get("eventType") != strconv.Itoa(int(EventIndexerQuery)) {
		t.Errorf("eventType = %q", q.Get("eventType"))
	}
}

// TestHistoryPagesByPageNumber walks to a short page.
func TestHistoryPagesByPageNumber(t *testing.T) {
	const pageSize = 2
	var pages []string

	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		pages = append(pages, page)

		var recs []HistoryRecord
		switch page {
		case "1":
			recs = []HistoryRecord{{ID: 1}, {ID: 2}}
		case "2":
			recs = []HistoryRecord{{ID: 3}}
		}
		_ = json.NewEncoder(w).Encode(historyPage{
			Page: 1, PageSize: pageSize, TotalRecords: 3, Records: recs,
		})
	}))

	var seen int
	if err := c.History(context.Background(), EventReleaseGrabbed, pageSize,
		func(recs []HistoryRecord) bool {
			seen += len(recs)
			return true
		}); err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 || pages[0] != "1" || pages[1] != "2" {
		t.Errorf("pages = %v, want [1 2]", pages)
	}
	if seen != 3 {
		t.Errorf("saw %d records, want 3", seen)
	}
}

func TestHistoryStopsOnCallbackFalse(t *testing.T) {
	var requests int
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_ = json.NewEncoder(w).Encode(historyPage{
			TotalRecords: 1000,
			Records:      []HistoryRecord{{ID: 1}, {ID: 2}},
		})
	}))

	if err := c.History(context.Background(), EventIndexerQuery, 2,
		func([]HistoryRecord) bool { return false }); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Errorf("made %d requests, want 1", requests)
	}
}

func TestHistoryEmptyPageTerminates(t *testing.T) {
	var requests int
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests > 5 {
			t.Fatal("the walk did not terminate")
		}
		_ = json.NewEncoder(w).Encode(historyPage{})
	}))
	if err := c.History(context.Background(), EventIndexerQuery, 10,
		func([]HistoryRecord) bool { return true }); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Errorf("made %d requests", requests)
	}
}

// TestHistoryRecordDecodesARealPayload pins the field names that matter.
func TestHistoryRecordDecodesARealPayload(t *testing.T) {
	const body = `{
	  "id": 71234,
	  "indexerId": 12,
	  "date": "2026-09-13T12:00:00Z",
	  "eventType": "indexerQuery",
	  "data": {
	    "query": "Some Show",
	    "queryResults": 42,
	    "elapsedTime": 318,
	    "cached": 0,
	    "imdbId": "tt0000001",
	    "tvdbId": 81189,
	    "source": "Sonarr",
	    "url": "https://tracker.example/api?passkey=deadbeefdeadbeefdead"
	  }
	}`
	var r HistoryRecord
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatal(err)
	}
	if r.ID != 71234 || r.IndexerID != 12 {
		t.Errorf("ids = %d / %d", r.ID, r.IndexerID)
	}
	if r.Date.IsZero() {
		t.Error("date did not decode")
	}
	if n, ok := r.Int("queryResults"); !ok || n != 42 {
		t.Errorf("queryResults = %d, %v", n, ok)
	}
	// A search that returned nothing is the interesting half of this corpus, so
	// a zero result count must be present-and-zero rather than absent.
	if v, ok := r.Bool("cached"); !ok || v {
		t.Errorf("cached = %v, %v", v, ok)
	}
	if r.Str("tvdbId") != "81189" {
		t.Errorf("tvdbId = %q; a numeric id must survive as text", r.Str("tvdbId"))
	}
}

func TestIndexersDecode(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{
		  "id": 12, "name": "TorrentLeech",
		  "protocol": "torrent", "privacy": "private", "priority": 25
		}]`))
	}))
	got, err := c.Indexers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d indexers", len(got))
	}
	if got[0].ID != 12 || got[0].Name != "TorrentLeech" || got[0].Privacy != "private" {
		t.Errorf("indexer = %+v", got[0])
	}
}

func TestAuthHeaderIsSent(t *testing.T) {
	var got string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Api-Key")
		_, _ = w.Write([]byte(`[]`))
	}))
	if _, err := c.Indexers(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got != "test-key" {
		t.Errorf("X-Api-Key = %q", got)
	}
}
