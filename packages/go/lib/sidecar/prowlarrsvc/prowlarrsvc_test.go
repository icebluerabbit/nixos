package prowlarrsvc

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
	"homelab/sidecar"
)

// newService points a service at a handler standing in for Prowlarr.
func newService(t *testing.T, h http.HandlerFunc) *Service {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := prowlarr.New(httpx.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return New(c, srv.URL)
}

// history serves a per-event-type corpus with real page/pageSize semantics.
// counts is keyed by Prowlarr's numeric eventType.
func history(t *testing.T, counts map[int]int, queries *[]string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "indexer") {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 1, "name": "tracker", "protocol": "torrent"},
			})
			return
		}
		if queries != nil {
			*queries = append(*queries, r.URL.RawQuery)
		}
		event, _ := strconv.Atoi(r.URL.Query().Get("eventType"))
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		size, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
		if page < 1 {
			page = 1
		}
		total := counts[event]

		records := []map[string]any{}
		for i := (page - 1) * size; i < page*size && i < total; i++ {
			records = append(records, map[string]any{
				"id": i, "indexerId": 1,
				"date":      "2026-01-01T00:00:00Z",
				"eventType": fmt.Sprintf("type%d", event),
				"data":      map[string]any{"title": fmt.Sprintf("e%d-%d", event, i)},
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": page, "pageSize": size,
			"totalRecords": total, "records": records,
		})
	}
}

func TestInfo(t *testing.T) {
	got := newService(t, history(t, nil, nil)).Info()

	if got.Sidecar != "managarr-prowlarr" || got.Source != Source {
		t.Errorf("info = %+v", got)
	}
	if len(got.Capabilities) != 2 {
		t.Errorf("capabilities = %v", got.Capabilities)
	}
}

// Ascending is what makes the offset usable: a record arriving mid-walk lands
// at the end and cannot shift a page already read. Prowlarr's own History walks
// descending for a different caller, so the direction has to be asserted here.
func TestChangesRequestsAscending(t *testing.T) {
	var queries []string
	svc := newService(t, history(t, map[int]int{1: 2}, &queries))

	if _, err := svc.Changes(context.Background(), "", 10); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(queries[0], "sortDirection=ascending") {
		t.Errorf("query = %q, want ascending", queries[0])
	}
}

// Both served event types have to be reachable from one walk. A stage that runs
// dry must advance rather than end it, or queries are never read — the consumer
// stops on a short page and cannot know there is more behind it.
//
// RSS is seeded here and must NOT come back. managarr records none of them, and
// at 44,000 records they outnumber real searches two to one, so paging them
// across the VLAN for a consumer that drops every one is pure waste.
func TestChangesCoversEveryStage(t *testing.T) {
	svc := newService(t, history(t, map[int]int{1: 3, 2: 2, 3: 99}, nil))
	ctx := context.Background()

	kinds := map[string]int{}
	cursor := ""
	for range 20 {
		events, err := svc.Changes(ctx, cursor, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) == 0 {
			break
		}
		for _, e := range events {
			kinds[e.Kind]++
		}
		cursor = events[len(events)-1].Cursor
	}

	for kind, want := range map[string]int{
		KindGrabbed: 3, KindQuery: 2, KindRSS: 0,
	} {
		if kinds[kind] != want {
			t.Errorf("%s = %d, want %d", kind, kinds[kind], want)
		}
	}
}

// The RSS endpoint must never be asked at all, not merely filtered afterwards.
// Fetching and discarding would cost the same bandwidth as serving it.
func TestChangesNeverRequestsRSS(t *testing.T) {
	var queries []string
	svc := newService(t, history(t, map[int]int{1: 1, 2: 1, 3: 99}, &queries))

	cursor := ""
	for range 10 {
		events, err := svc.Changes(context.Background(), cursor, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) == 0 {
			break
		}
		cursor = events[len(events)-1].Cursor
	}

	for _, q := range queries {
		if strings.Contains(q, "eventType=3") {
			t.Errorf("asked Prowlarr for RSS polls: %q", q)
		}
	}
}

// An empty stage must not end the walk. Grabs are first, so a Prowlarr that has
// proxied searches but never had one grabbed through it would otherwise report
// nothing at all.
func TestChangesSkipsAnEmptyStage(t *testing.T) {
	svc := newService(t, history(t, map[int]int{1: 0, 2: 2, 3: 0}, nil))
	ctx := context.Background()

	kinds := map[string]int{}
	cursor := ""
	for range 20 {
		events, err := svc.Changes(ctx, cursor, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) == 0 {
			break
		}
		for _, e := range events {
			kinds[e.Kind]++
		}
		cursor = events[len(events)-1].Cursor
	}

	if kinds[KindQuery] != 2 {
		t.Errorf("an empty first stage ended the walk: %v", kinds)
	}
}

func TestChangesEventShape(t *testing.T) {
	events, err := newService(t, history(t, map[int]int{1: 1}, nil)).
		Changes(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("no events")
	}

	e := events[0]
	if e.Source != Source || e.Kind != KindGrabbed {
		t.Errorf("event = %s/%s", e.Source, e.Kind)
	}
	// Prowlarr never sees an infohash — it hands over a link — so the title
	// is the only identifier it shares with the rest of the stack.
	if e.Ref != "e1-0" {
		t.Errorf("ref = %q", e.Ref)
	}
	if e.OccurredAt.IsZero() {
		t.Error("date was lost")
	}

	got, err := sidecar.DecodePayload[prowlarr.HistoryRecord](e)
	if err != nil {
		t.Fatal(err)
	}
	if got.IndexerID != 1 {
		t.Errorf("payload = %+v", got)
	}
}

// The stage boundary has to land on the last event of the page, or a consumer
// that stops there resumes at the end of a finished stage and reads it again.
func TestChangesRecordsTheStageBoundary(t *testing.T) {
	svc := newService(t, history(t, map[int]int{1: 2, 2: 2, 3: 0}, nil))
	ctx := context.Background()

	first, err := svc.Changes(ctx, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 {
		t.Fatalf("got %d events, want 2", len(first))
	}
	// Stage 0 held exactly 2, so the cursor must already point at stage 1.
	if first[1].Cursor != "1:0" {
		t.Errorf("cursor = %q, want 1:0", first[1].Cursor)
	}

	second, err := svc.Changes(ctx, first[1].Cursor, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 2 || second[0].Kind != KindQuery {
		t.Errorf("resumed into %+v, want the query stage", second)
	}
}

// ref falls through the data bag rather than assuming a shape: which keys a
// record carries depends on its event type and none is guaranteed.
func TestRefFallsThroughTheDataBag(t *testing.T) {
	tests := []struct {
		name string
		data map[string]string
		want string
	}{
		{"title", map[string]string{"title": "T"}, "T"},
		{"query when no title", map[string]string{"query": "Q"}, "Q"},
		{"prefers title", map[string]string{"title": "T", "query": "Q"}, "T"},
		{"trims", map[string]string{"title": "  T  "}, "T"},
		{"skips an empty value", map[string]string{"title": "", "query": "Q"}, "Q"},
		{"nothing usable", map[string]string{"other": "x"}, ""},
		{"empty bag", nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ref(prowlarr.HistoryRecord{Data: tc.data})
			if got != tc.want {
				t.Errorf("ref = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestChangesBadCursor(t *testing.T) {
	svc := newService(t, history(t, map[int]int{1: 1}, nil))

	for _, c := range []string{"abc", "nocolon", "-1:0"} {
		if _, err := svc.Changes(context.Background(), c, 10); err == nil {
			t.Errorf("cursor %q was accepted", c)
		}
	}
}

// A cursor past the last stage is the end of the walk, not an error: it is
// exactly what the previous call handed back.
func TestChangesPastTheLastStage(t *testing.T) {
	events, err := newService(t, history(t, map[int]int{1: 1}, nil)).
		Changes(context.Background(), "3:0", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("got %d events, want 0", len(events))
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

func TestEnrich(t *testing.T) {
	const body = `{"eventType":"Grab","release":{"releaseTitle":"Some.Release"}}`

	events, err := newService(t, history(t, nil, nil)).
		Enrich(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Kind != KindWebhook || events[0].Ref != "Some.Release" {
		t.Errorf("event = %+v", events[0])
	}
	if string(events[0].Payload) != body {
		t.Errorf("payload = %s, want it unchanged", events[0].Payload)
	}
}

func TestEnrichRejectsBadBodies(t *testing.T) {
	svc := newService(t, history(t, nil, nil))

	for name, body := range map[string]string{
		"not json":     "nonsense",
		"no eventType": `{"release":{"releaseTitle":"R"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.Enrich(context.Background(), []byte(body)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

// A history record names its indexer by id and nothing else, so without this
// every record is attributed to a number.
func TestIndexersRoute(t *testing.T) {
	svc := newService(t, history(t, nil, nil))
	mux := http.NewServeMux()
	svc.Routes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, IndexersPath, http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}

	var body struct {
		Indexers []prowlarr.Indexer `json:"indexers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Indexers) != 1 || body.Indexers[0].Name != "tracker" {
		t.Errorf("indexers = %+v", body.Indexers)
	}
}

func TestIndexersRouteReportsFailure(t *testing.T) {
	svc := newService(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux := http.NewServeMux()
	svc.Routes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, IndexersPath, http.NoBody))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestImplementsContract(t *testing.T) {
	var (
		_ sidecar.Service  = (*Service)(nil)
		_ sidecar.Extender = (*Service)(nil)
	)
}
