package arrsvc

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
	"homelab/sidecar"
)

// newService points a service at a handler standing in for a Servarr instance.
func newService(t *testing.T, h http.HandlerFunc) *Service {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := arr.New(arr.Sonarr, httpx.Config{BaseURL: srv.URL}, "key")
	if err != nil {
		t.Fatal(err)
	}
	return New(c, srv.URL)
}

// history serves n records with real page/pageSize semantics, so the walk runs
// against what a *arr actually does rather than a handler that ignores the
// query.
func history(t *testing.T, n int, queries *[]string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if queries != nil {
			*queries = append(*queries, r.URL.RawQuery)
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		size, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
		if page < 1 {
			page = 1
		}

		records := []arr.HistoryRecord{}
		for i := (page - 1) * size; i < page*size && i < n; i++ {
			records = append(records, arr.HistoryRecord{
				ID:          int64(i),
				EventType:   "grabbed",
				SourceTitle: fmt.Sprintf("Release.%d", i),
				DownloadID:  fmt.Sprintf("%040X", i),
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": page, "pageSize": size,
			"totalRecords": n, "records": records,
		})
	}
}

func TestInfo(t *testing.T) {
	got := newService(t, func(http.ResponseWriter, *http.Request) {}).Info()

	if got.Sidecar != "managarr-sonarr" || got.Source != "sonarr" {
		t.Errorf("info = %+v", got)
	}
	if len(got.Capabilities) != 2 {
		t.Errorf("capabilities = %v, want changes and events", got.Capabilities)
	}
}

// Info must not touch the upstream: it is the diagnostic you reach for when the
// upstream is the thing that is wrong.
func TestInfoWithUpstreamDown(t *testing.T) {
	svc := newService(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	if got := svc.Info(); got.Source != "sonarr" {
		t.Errorf("info = %+v", got)
	}
}

// Ascending is what makes an offset cursor usable at all: history is appended,
// so a record arriving mid-walk lands at the end and cannot shift a page
// already read. A descending sort would silently skip records.
func TestChangesRequestsAscending(t *testing.T) {
	var queries []string
	svc := newService(t, history(t, 3, &queries))

	if _, err := svc.Changes(context.Background(), "", 10); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(queries[0], "sortDirection=ascending") {
		t.Errorf("query = %q, want ascending", queries[0])
	}
	if !strings.Contains(queries[0], "sortKey=date") {
		t.Errorf("query = %q, want sortKey=date", queries[0])
	}
}

func TestChanges(t *testing.T) {
	events, err := newService(t, history(t, 3, nil)).Changes(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}

	e := events[0]
	if e.Source != "sonarr" || e.Kind != KindHistory {
		t.Errorf("event = %s/%s", e.Source, e.Kind)
	}
	// The download id is what ties a history record to a torrent.
	if e.Ref != fmt.Sprintf("%040X", 0) {
		t.Errorf("ref = %q", e.Ref)
	}
	if e.Cursor != "1" {
		t.Errorf("cursor = %q, want 1 — the count consumed, not the index", e.Cursor)
	}

	got, err := sidecar.DecodePayload[arr.HistoryRecord](e)
	if err != nil {
		t.Fatal(err)
	}
	if got.SourceTitle != "Release.0" {
		t.Errorf("payload = %+v", got)
	}
}

func TestChangesResumes(t *testing.T) {
	svc := newService(t, history(t, 25, nil))
	ctx := context.Background()

	first, err := svc.Changes(ctx, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 10 || first[9].Cursor != "10" {
		t.Fatalf("first page = %d events ending %q", len(first), first[len(first)-1].Cursor)
	}

	second, err := svc.Changes(ctx, first[9].Cursor, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 10 {
		t.Fatalf("second page = %d events, want 10", len(second))
	}
	payload, err := sidecar.DecodePayload[arr.HistoryRecord](second[0])
	if err != nil {
		t.Fatal(err)
	}
	if payload.SourceTitle != "Release.10" {
		t.Errorf("resumed at %q, want Release.10", payload.SourceTitle)
	}
}

func TestChangesEndOfHistory(t *testing.T) {
	svc := newService(t, history(t, 25, nil))

	events, err := svc.Changes(context.Background(), "20", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 5 {
		t.Errorf("got %d events, want the 5 remaining", len(events))
	}

	done, err := svc.Changes(context.Background(), "25", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 0 {
		t.Errorf("got %d events past the end, want 0", len(done))
	}
}

// A consumer is free to change its limit between calls, which leaves the offset
// off a page boundary. Returning a short page then would end its walk early,
// because a short page has to mean "the history ended" and nothing else.
func TestChangesUnalignedOffset(t *testing.T) {
	svc := newService(t, history(t, 100, nil))

	events, err := svc.Changes(context.Background(), "7", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 10 {
		t.Fatalf("got %d events from an unaligned offset, want a full 10", len(events))
	}

	first, err := sidecar.DecodePayload[arr.HistoryRecord](events[0])
	if err != nil {
		t.Fatal(err)
	}
	if first.SourceTitle != "Release.7" {
		t.Errorf("started at %q, want Release.7", first.SourceTitle)
	}
	last, err := sidecar.DecodePayload[arr.HistoryRecord](events[9])
	if err != nil {
		t.Fatal(err)
	}
	if last.SourceTitle != "Release.16" {
		t.Errorf("ended at %q, want Release.16", last.SourceTitle)
	}
	if events[9].Cursor != "17" {
		t.Errorf("cursor = %q, want 17", events[9].Cursor)
	}
}

func TestChangesBadCursor(t *testing.T) {
	svc := newService(t, history(t, 3, nil))

	for _, c := range []string{"abc", "-1", "1.5"} {
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

// enrichService serves a queue containing one item for downloadID.
func enrichService(t *testing.T, downloadID string, queueFails bool) *Service {
	t.Helper()
	return newService(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "queue") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if queueFails {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": 1, "pageSize": 1, "totalRecords": 1,
			"records": []map[string]any{{
				"id": 1, "downloadId": downloadID,
				"title": "What.The.Client.Called.It",
				"size":  1234.0,
			}},
		})
	})
}

// The whole reason enrichment happens in the sidecar: the queue record exists
// for the minutes between a grab and an import, and an hourly pass is not there
// to see it.
func TestEnrichGrabAttachesQueue(t *testing.T) {
	const id = "ABC123"
	svc := enrichService(t, id, false)

	events, err := svc.Enrich(context.Background(),
		[]byte(`{"eventType":"Grab","downloadId":"`+id+`","release":{"releaseTitle":"R"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Ref != id {
		t.Errorf("ref = %q, want %q", events[0].Ref, id)
	}

	var got enriched
	if err := json.Unmarshal(events[0].Payload, &got); err != nil {
		t.Fatal(err)
	}
	if got.Queue == nil {
		t.Fatal("queue was not attached")
	}
	if got.Queue.Title != "What.The.Client.Called.It" {
		t.Errorf("queue title = %q", got.Queue.Title)
	}
	if got.Webhook.Release.ReleaseTitle != "R" {
		t.Errorf("webhook was not preserved: %+v", got.Webhook)
	}
}

// Matched case-insensitively. The *arrs report the download id uppercase and
// most other things in this stack lowercase it, and a mismatch here would just
// silently drop the enrichment.
func TestEnrichGrabMatchesCaseInsensitively(t *testing.T) {
	svc := enrichService(t, "abc123", false)

	events, err := svc.Enrich(context.Background(),
		[]byte(`{"eventType":"Grab","downloadId":"ABC123"}`))
	if err != nil {
		t.Fatal(err)
	}
	var got enriched
	if err := json.Unmarshal(events[0].Payload, &got); err != nil {
		t.Fatal(err)
	}
	if got.Queue == nil {
		t.Error("case difference lost the queue match")
	}
}

// A failed lookup must not drop the event: the notification is still worth
// relaying without the queue detail.
func TestEnrichGrabSurvivesQueueFailure(t *testing.T) {
	svc := enrichService(t, "ABC", true)

	events, err := svc.Enrich(context.Background(),
		[]byte(`{"eventType":"Grab","downloadId":"ABC"}`))
	if err != nil {
		t.Fatalf("a queue failure must not fail the event: %v", err)
	}
	var got enriched
	if err := json.Unmarshal(events[0].Payload, &got); err != nil {
		t.Fatal(err)
	}
	if got.Queue != nil {
		t.Error("queue should be absent")
	}
	if got.Webhook.EventType != "Grab" {
		t.Error("webhook was lost")
	}
}

// Only grabs are enriched. An import has already left the queue, so a lookup
// there is a wasted round trip on every imported file.
func TestEnrichImportDoesNotCallQueue(t *testing.T) {
	called := false
	svc := newService(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "queue") {
			called = true
		}
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := svc.Enrich(context.Background(),
		[]byte(`{"eventType":"Download","downloadId":"ABC","episodeFile":{"path":"/x.mkv"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("an import should not hit the queue")
	}
}

func TestEnrichRejectsBadBodies(t *testing.T) {
	svc := newService(t, func(http.ResponseWriter, *http.Request) {})

	t.Run("not json", func(t *testing.T) {
		if _, err := svc.Enrich(context.Background(), []byte("nonsense")); err == nil {
			t.Fatal("expected an error")
		}
	})
	// An eventType is the one field every Servarr notification has. A body
	// without one is not a notification, and relaying it would put a row in
	// managarr that nothing can interpret.
	t.Run("no eventType", func(t *testing.T) {
		if _, err := svc.Enrich(context.Background(), []byte(`{"downloadId":"A"}`)); err == nil {
			t.Fatal("expected an error")
		}
	})
}

// A health notification has no download id, and that is normal rather than a
// failure — it is also the only record of a check that flared and cleared
// between two passes.
func TestEnrichHealthHasNoRef(t *testing.T) {
	svc := newService(t, func(http.ResponseWriter, *http.Request) {})

	events, err := svc.Enrich(context.Background(),
		[]byte(`{"eventType":"Health","level":"warning","message":"m"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Ref != "" {
		t.Errorf("events = %+v", events)
	}
}

func TestImplementsContract(t *testing.T) {
	var _ sidecar.Service = (*Service)(nil)
}
