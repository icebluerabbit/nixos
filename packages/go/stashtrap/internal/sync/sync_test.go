package sync_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"homelab/clients/stashbox"
	"homelab/secret"
	"homelab/stashtrap/internal/store"
	"homelab/stashtrap/internal/sync"
)

// newStub serves the queries the syncer makes, dispatching on the operation
// text because that is what distinguishes them on the wire.
func newStub(t *testing.T, handler func(string, map[string]any) string) (*stashbox.Client, *httptest.Server, func() int) {
	t.Helper()

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("ApiKey") != "test-key" {
			t.Errorf("missing ApiKey header: %v", r.Header)
		}
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, handler(body.Query, body.Variables)) //nolint:errcheck // test stub
	}))
	t.Cleanup(srv.Close)

	client, err := stashbox.New(stashbox.Config{
		Endpoint: srv.URL,
		APIKey:   secret.New("test-key"),
		// The real floor would make every test wait a second per request.
		MinInterval: time.Microsecond,
	})
	if err != nil {
		t.Fatalf("stashbox.New: %v", err)
	}
	t.Cleanup(client.Close)
	return client, srv, func() int { return calls }
}

func newSyncer(t *testing.T, client *stashbox.Client) (*sync.Syncer, *store.Store) {
	t.Helper()

	st, err := store.Open(filepath.Join(t.TempDir(), "stashtrap.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	s, err := sync.New(sync.Deps{
		Client:   client,
		Store:    st,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Endpoint: "https://stashdb.test/graphql",
		PerPage:  2,
	})
	if err != nil {
		t.Fatalf("sync.New: %v", err)
	}
	return s, st
}

func TestRunFillsCatalogue(t *testing.T) {
	t.Parallel()

	client, _, calls := newStub(t, func(q string, v map[string]any) string {
		page, _ := v["page"].(float64)
		switch {
		// Two pages of favourites, to prove pagination terminates on the
		// reported count rather than on a short page.
		case strings.Contains(q, "queryPerformers"):
			if page == 1 {
				return `{"data":{"queryPerformers":{"count":3,"performers":[
					{"id":"p1","name":"Jane Doe","aliases":["jane.doe"],"is_favorite":true},
					{"id":"p2","name":"Other Person","aliases":[],"is_favorite":true}]}}}`
			}
			return `{"data":{"queryPerformers":{"count":3,"performers":[
				{"id":"p3","name":"Third","aliases":[],"is_favorite":true}]}}}`
		case strings.Contains(q, "queryStudios"):
			return `{"data":{"queryStudios":{"count":1,"studios":[
				{"id":"s1","name":"ExampleStudio","aliases":["ExampleStudio.com"],"is_favorite":true,
				 "parent":{"id":"net1","name":"Network"}}]}}}`
		case strings.Contains(q, "queryScenes"):
			return `{"data":{"queryScenes":{"count":1,"scenes":[
				{"id":"sc1","title":"A Scene","release_date":"2026-09-01","code":"TS1",
				 "studio":{"id":"s1","name":"ExampleStudio"},
				 "performers":[{"performer":{"id":"p1","name":"Jane Doe"},"as":"Jane D"}],
				 "fingerprints":[{"hash":"abc","algorithm":"OSHASH","duration":1800,"submissions":2}],
				 "tags":[{"id":"t1","name":"Vintage"}]}]}}}`
		}
		t.Errorf("unexpected query: %s", q)
		return `{"data":{}}`
	})

	s, st := newSyncer(t, client)
	res, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Performers != 3 || res.Studios != 1 {
		t.Fatalf("Result = %+v", res)
	}
	// One scene query per followed entity: three performers plus one studio.
	if res.Scenes != 4 {
		t.Fatalf("scenes = %d, want 4 (one per followed entity)", res.Scenes)
	}

	perf, studios, scenes, err := st.CatalogueCounts(context.Background(), "https://stashdb.test/graphql")
	if err != nil {
		t.Fatalf("CatalogueCounts: %v", err)
	}
	// The same scene arriving four times is still one row.
	if perf != 3 || studios != 1 || scenes != 1 {
		t.Fatalf("counts = %d/%d/%d, want 3/1/1", perf, studios, scenes)
	}
	if calls() == 0 {
		t.Fatal("no requests made")
	}
}

// A crawl that fails partway records the error rather than reporting success.
func TestRunReportsFailure(t *testing.T) {
	t.Parallel()

	client, _, _ := newStub(t, func(string, map[string]any) string {
		return `{"errors":[{"message":"rate limited"}]}`
	})

	s, _ := newSyncer(t, client)
	if _, err := s.Run(context.Background()); err == nil {
		t.Fatal("Run succeeded against a failing service")
	}
}

// The timer can fire while a long first fill is still running; a second crawl
// against the same tables and rate limit must be refused, not queued.
func TestRunSingleFlight(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	client, _, _ := newStub(t, func(q string, _ map[string]any) string {
		switch {
		case strings.Contains(q, "queryPerformers"):
			<-release
			return `{"data":{"queryPerformers":{"count":0,"performers":[]}}}`
		case strings.Contains(q, "queryStudios"):
			return `{"data":{"queryStudios":{"count":0,"studios":[]}}}`
		default:
			return `{"data":{"queryScenes":{"count":0,"scenes":[]}}}`
		}
	})

	s, _ := newSyncer(t, client)
	done := make(chan error, 1)
	go func() {
		_, err := s.Run(context.Background())
		done <- err
	}()

	// Wait until the first crawl is definitely inside the stub.
	for !s.Running() {
		runtime.Gosched()
	}
	if _, err := s.Run(context.Background()); !errors.Is(err, sync.ErrInProgress) {
		t.Fatalf("second Run = %v, want ErrInProgress", err)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first Run: %v", err)
	}
}

func TestNewRequiresDeps(t *testing.T) {
	t.Parallel()

	if _, err := sync.New(sync.Deps{Endpoint: "x"}); err == nil {
		t.Fatal("New succeeded without a client or store")
	}
}

// The catalogue gauges are what a dashboard reads to tell a stale catalogue
// from an empty one, so they have to be published after a crawl.
func TestRunPublishesMetrics(t *testing.T) {
	t.Parallel()

	client, _, _ := newStub(t, func(q string, _ map[string]any) string {
		switch {
		case strings.Contains(q, "queryPerformers"):
			return `{"data":{"queryPerformers":{"count":1,"performers":[{"id":"p1","name":"A","is_favorite":true}]}}}`
		case strings.Contains(q, "queryStudios"):
			return `{"data":{"queryStudios":{"count":0,"studios":[]}}}`
		default:
			return `{"data":{"queryScenes":{"count":1,"scenes":[{"id":"sc1","title":"S"}]}}}`
		}
	})

	st, err := store.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	reg := prometheus.NewRegistry()
	s, err := sync.New(sync.Deps{
		Client:   client,
		Store:    st,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Endpoint: "https://stashdb.test/graphql",
		PerPage:  2,
		Registry: reg,
	})
	if err != nil {
		t.Fatalf("sync.New: %v", err)
	}
	if _, err := s.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := gather(t, reg)

	if got["stashtrap_catalogue_entities/performers"] != 1 {
		t.Fatalf("performer gauge = %v; all: %v", got["stashtrap_catalogue_entities/performers"], got)
	}
	if got["stashtrap_catalogue_entities/scenes"] != 1 {
		t.Fatalf("scene gauge = %v", got["stashtrap_catalogue_entities/scenes"])
	}
	// Running must be cleared once the crawl returns, or a dashboard shows a
	// crawl that ended hours ago as still in flight.
	if got["stashtrap_sync_running"] != 0 {
		t.Fatalf("running gauge = %v, want 0", got["stashtrap_sync_running"])
	}
	if got["stashtrap_sync_last_success_timestamp_seconds"] == 0 {
		t.Fatal("last success timestamp not set")
	}
	if got["stashtrap_sync_failures_total"] != 0 {
		t.Fatalf("failures = %v, want 0", got["stashtrap_sync_failures_total"])
	}
}

// A failed crawl has to increment the failure counter: a dashboard showing a
// stale catalogue with no failures is reporting the wrong thing.
func TestRunCountsFailures(t *testing.T) {
	t.Parallel()

	client, _, _ := newStub(t, func(string, map[string]any) string {
		return `{"errors":[{"message":"rate limited"}]}`
	})

	st, err := store.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	reg := prometheus.NewRegistry()
	s, err := sync.New(sync.Deps{
		Client:   client,
		Store:    st,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Endpoint: "https://stashdb.test/graphql",
		Registry: reg,
	})
	if err != nil {
		t.Fatalf("sync.New: %v", err)
	}
	if _, err := s.Run(context.Background()); err == nil {
		t.Fatal("Run succeeded against a failing service")
	}

	got := gather(t, reg)
	if got["stashtrap_sync_failures_total"] != 1 {
		t.Fatalf("failures = %v, want 1", got["stashtrap_sync_failures_total"])
	}
	if got["stashtrap_sync_running"] != 0 {
		t.Fatalf("running gauge = %v after failure, want 0", got["stashtrap_sync_running"])
	}
}

// The catalogue stores a studio under the name in its own URL as well as the
// name stash-box calls it, because a release writes the domain: "FirstSite.com"
// where StashDB says "First Site".
func TestRunDerivesStudioDomainAliases(t *testing.T) {
	t.Parallel()

	client, _, _ := newStub(t, func(q string, _ map[string]any) string {
		switch {
		case strings.Contains(q, "queryStudios"):
			return `{"data":{"queryStudios":{"count":1,"studios":[
				{"id":"s1","name":"First Site","aliases":[],"is_favorite":false,
				 "urls":[{"url":"https://www.firstsite.com/","site":{"name":"Home"}},
				         {"url":"https://directory.example.com/firstsite","site":{"name":"Elsewhere"}}]}]}}}`
		case strings.Contains(q, "queryPerformers"):
			return `{"data":{"queryPerformers":{"count":0,"performers":[]}}}`
		}
		return `{"data":{"queryScenes":{"count":0,"scenes":[]}}}`
	})

	s, st := newSyncer(t, client)
	if _, err := s.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	studios := func(probe string) []string {
		t.Helper()
		hits, err := st.Lookup(context.Background(), "https://stashdb.test/graphql", []string{probe})
		if err != nil {
			t.Fatalf("Lookup(%q): %v", probe, err)
		}
		var ids []string
		for _, h := range hits {
			if h.Kind == store.KindStudio {
				ids = append(ids, h.ID)
			}
		}
		return ids
	}

	if ids := studios("firstsite"); len(ids) != 1 || ids[0] != "s1" {
		t.Fatalf("studios answering to \"firstsite\" = %v, want [s1]", ids)
	}
	// The directory link is not the studio's own site, and aliasing every
	// studio listed there to "directory" would match the wrong thing.
	for _, alias := range []string{"directory", "directoryexample"} {
		if ids := studios(alias); len(ids) != 0 {
			t.Fatalf("directory host stored as an alias %q: %v", alias, ids)
		}
	}
}

// gather flattens a registry into name/label -> value, which is enough to
// assert on without hand-walking protobufs in every test.
func gather(t *testing.T, reg *prometheus.Registry) map[string]float64 {
	t.Helper()

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	out := map[string]float64{}
	for _, f := range families {
		for _, m := range f.GetMetric() {
			key := f.GetName()
			for _, l := range m.GetLabel() {
				key += "/" + l.GetValue()
			}
			switch {
			case m.GetGauge() != nil:
				out[key] = m.GetGauge().GetValue()
			case m.GetCounter() != nil:
				out[key] = m.GetCounter().GetValue()
			}
		}
	}
	return out
}
