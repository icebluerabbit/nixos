package server_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zeebo/bencode"

	"homelab/app"
	"homelab/stashtrap/internal/policy"
	"homelab/stashtrap/internal/server"
	"homelab/stashtrap/internal/store"
)

var appEnv = app.Env{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Metrics: prometheus.NewRegistry()}

type harness struct {
	handler http.Handler
	store   *store.Store
	reg     *prometheus.Registry
	logs    *bytes.Buffer
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	st, err := store.Open(filepath.Join(t.TempDir(), "stashtrap.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	logs := &bytes.Buffer{}
	reg := prometheus.NewRegistry()
	srv, err := server.New(server.Deps{
		Store:    st,
		Decider:  policy.New(),
		Logger:   slog.New(slog.NewTextHandler(logs, nil)),
		Endpoint: catalogueEndpoint,
		Now:      func() time.Time { return time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC) },
	}, reg)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	return &harness{handler: srv.Handler(), store: st, reg: reg, logs: logs}
}

// catalogueEndpoint namespaces the catalogue rows a match test seeds.
const catalogueEndpoint = "https://stashdb.test/graphql"

// A release naming a followed performer is what this service exists to grab,
// and the log line has to explain why on its own.
func TestAnnounceAcceptsFavouritePerformer(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	if err := h.store.UpsertPerformer(context.Background(), catalogueEndpoint, store.Performer{
		ID:          "p1",
		Name:        "Jane Doe",
		Aliases:     []string{"jane.doe"},
		IsFavourite: true,
	}); err != nil {
		t.Fatalf("UpsertPerformer: %v", err)
	}

	rec := h.post(t, `{"name":"[SiteOne.example] jane.doe - A Title 1080p","indexer":"bitporn"}`)
	if rec.Code != server.StatusAccept {
		t.Fatalf("status = %d, want %d for a followed performer", rec.Code, server.StatusAccept)
	}

	logs := h.logs.String()
	for _, want := range []string{
		"decision=accept",
		"reason=" + policy.ReasonFavouritePerformer,
		"favourite_performers=[p1]",
		"names=[janedoe]",
	} {
		if !strings.Contains(logs, want) {
			t.Fatalf("log missing %q: %s", want, logs)
		}
	}
}

// The dashboards ask who a grab was for, which the announce counter cannot
// answer, so the reading is measured separately and labelled by name.
func TestAnnounceMeasuresTheReading(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	if err := h.store.UpsertPerformer(context.Background(), catalogueEndpoint, store.Performer{
		ID:          "p1",
		Name:        "Jane Doe",
		Aliases:     []string{"jane.doe"},
		IsFavourite: true,
	}); err != nil {
		t.Fatalf("UpsertPerformer: %v", err)
	}

	h.post(t, `{"name":"[SiteOne.example] jane.doe - A Title 1080p","indexer":"bitporn"}`)

	families, err := h.reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	got := map[string]float64{}
	for _, f := range families {
		for _, m := range f.GetMetric() {
			key := f.GetName()
			for _, l := range m.GetLabel() {
				key += "/" + l.GetValue()
			}
			if c := m.GetCounter(); c != nil {
				got[key] = c.GetValue()
			}
			if hist := m.GetHistogram(); hist != nil {
				got[key] = float64(hist.GetSampleCount())
			}
		}
	}

	// Labelled by name, since a panel of ids tells nobody anything.
	if got["stashtrap_favourite_hits_total/performer/Jane Doe"] != 1 {
		t.Fatalf("favourite hit not measured by name: %v", got)
	}
	if got["stashtrap_recognised_total/performer"] != 1 {
		t.Fatalf("recognised performer not counted: %v", got)
	}
	if got["stashtrap_reading_score"] != 1 {
		t.Fatalf("reading score not observed: %v", got)
	}
}

// A performer the catalogue knows but the account does not follow is not a
// reason to grab anything.
func TestAnnounceRejectsPerformerWhoIsNotFollowed(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	if err := h.store.UpsertPerformer(context.Background(), catalogueEndpoint, store.Performer{
		ID:      "p2",
		Name:    "Second Performer",
		Aliases: []string{"second.performer"},
	}); err != nil {
		t.Fatalf("UpsertPerformer: %v", err)
	}

	rec := h.post(t, `{"name":"[SiteOne.example] second.performer - A Title 1080p","indexer":"bitporn"}`)
	if rec.Code != server.StatusReject {
		t.Fatalf("status = %d, want %d for a performer nobody follows", rec.Code, server.StatusReject)
	}
	if !strings.Contains(h.logs.String(), "reason="+policy.ReasonNoFavourite) {
		t.Fatalf("log does not explain the reject: %s", h.logs.String())
	}
}

// Before a sync there is nothing to recognise anyone against, and that has to
// be distinguishable from a working catalogue that found nobody.
func TestAnnounceWithEmptyCatalogueRecordsOnly(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	rec := h.post(t, `{"name":"[SiteOne.example] jane.doe - A Title","indexer":"bitporn"}`)
	if rec.Code != server.StatusReject {
		t.Fatalf("status = %d, want %d", rec.Code, server.StatusReject)
	}
	if !strings.Contains(h.logs.String(), "reason="+policy.ReasonRecordOnly) {
		t.Fatalf("empty catalogue did not report itself: %s", h.logs.String())
	}
}

// An announce whose name resolves to nothing logs no reading, rather than
// logging empty groups that read as a failed lookup.
func TestAnnounceWithoutMatch(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.post(t, `{"name":"Some Unrelated Release 2160p","indexer":"bitporn"}`)

	for _, unwanted := range []string{"score=", "performer_ids=", "studio_ids="} {
		if strings.Contains(h.logs.String(), unwanted) {
			t.Fatalf("reading logged for an unresolved name (%s): %s", unwanted, h.logs.String())
		}
	}
}

func (h *harness) post(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/announce", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

// torrentFixture builds a small multi-file torrent, base64'd the way the
// autobrr template sends it.
func torrentFixture(t *testing.T) string {
	t.Helper()
	infoRaw, err := bencode.EncodeBytes(map[string]any{
		"name": "Actress Pack",
		"files": []any{
			map[string]any{"path": []any{"a.mp4"}, "length": int64(10)},
			map[string]any{"path": []any{"b.mp4"}, "length": int64(20)},
		},
	})
	if err != nil {
		t.Fatalf("encode info: %v", err)
	}
	raw, err := bencode.EncodeBytes(map[string]any{"info": bencode.RawMessage(infoRaw)})
	if err != nil {
		t.Fatalf("encode metainfo: %v", err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// The whole contract with autobrr is the status code, so this is the test that
// matters most: a rejected release must not answer 250.
func TestAnnounceRejects(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	rec := h.post(t, `{"name":"release","indexer":"bitporn","size":100}`)

	if rec.Code != server.StatusReject {
		t.Fatalf("status = %d, want %d", rec.Code, server.StatusReject)
	}
	if rec.Code == server.StatusAccept {
		t.Fatal("phase one answered accept")
	}
	if n, err := h.store.Count(context.Background()); err != nil || n != 1 {
		t.Fatalf("Count = %d, %v; the announce was not recorded", n, err)
	}
}

func TestAnnounceRecordsTorrentFacts(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	body, err := json.Marshal(map[string]any{
		"name":         "[Site] Pack",
		"indexer":      "bitporn",
		"torrent_data": torrentFixture(t),
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if rec := h.post(t, string(body)); rec.Code != server.StatusReject {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(h.logs.String(), "files=2") {
		t.Fatalf("expected the file count in the log record: %s", h.logs.String())
	}
}

// A torrent that will not parse is still an announce worth recording: a
// tracker changing its output should show as data, not as a gap.
func TestAnnounceRecordsUnparseableTorrent(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	body, err := json.Marshal(map[string]any{
		"name":         "broken",
		"indexer":      "bitporn",
		"torrent_data": base64.StdEncoding.EncodeToString([]byte("not bencode")),
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if rec := h.post(t, string(body)); rec.Code != server.StatusReject {
		t.Fatalf("status = %d", rec.Code)
	}
	if n, err := h.store.Count(context.Background()); err != nil || n != 1 {
		t.Fatalf("Count = %d, %v", n, err)
	}
}

// A malformed body is a template bug, not a release: 400, and no row, because
// recording it would poison the corpus.
func TestAnnounceMalformedBody(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	rec := h.post(t, `{not json`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if n, err := h.store.Count(context.Background()); err != nil || n != 0 {
		t.Fatalf("Count = %d, %v; a malformed body was recorded", n, err)
	}
}

func TestAnnounceMetrics(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.post(t, `{"name":"a","indexer":"bitporn","size":100}`)
	h.post(t, `{"name":"b","indexer":"bitporn","size":200}`)

	families, err := h.reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	var total float64
	for _, f := range families {
		if f.GetName() != "stashtrap_announces_total" {
			continue
		}
		for _, m := range f.GetMetric() {
			total += m.GetCounter().GetValue()
			var indexer, decision, reason string
			for _, l := range m.GetLabel() {
				switch l.GetName() {
				case "indexer":
					indexer = l.GetValue()
				case "decision":
					decision = l.GetValue()
				case "reason":
					reason = l.GetValue()
				}
			}
			if indexer != "bitporn" || decision != "reject" || reason != policy.ReasonRecordOnly {
				t.Fatalf("labels = %q/%q/%q", indexer, decision, reason)
			}
		}
	}
	if total != 2 {
		t.Fatalf("stashtrap_announces_total = %v, want 2", total)
	}
}

func TestHealthAndReady(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	for _, path := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
		rec := httptest.NewRecorder()
		h.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d, want 200", path, rec.Code)
		}
	}
}

// Readiness has to fail once the database is gone: a unit that reports ready
// while it cannot record is worse than one that reports nothing.
func TestReadyFailsWithClosedStore(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	if err := h.store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestNewRequiresDeps(t *testing.T) {
	t.Parallel()

	if _, err := server.New(server.Deps{Decider: policy.New()}, prometheus.NewRegistry()); err == nil {
		t.Fatal("New succeeded without a store")
	}

	st, err := store.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	if _, err := server.New(server.Deps{Store: st}, prometheus.NewRegistry()); err == nil {
		t.Fatal("New succeeded without a decider")
	}
}

// Serve must return when the context is cancelled, or systemd's stop turns
// into a SIGKILL after the timeout.
func TestServeShutsDown(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	srv, err := server.New(server.Deps{
		Store:   h.store,
		Decider: policy.New(),
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, prometheus.NewRegistry())
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(ctx, &appEnv, srv, server.Config{
			Addr:            "127.0.0.1:0",
			ShutdownTimeout: time.Second,
		})
	}()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after cancellation")
	}
}

// Categories are their own counter rather than labels on announces_total: an
// announce carries two of them, so folding them in would double every other
// series and make a plain announce total wrong.
func TestAnnounceCategoryMetrics(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.post(t, `{"name":"a","indexer":"bitporn","categories":["XXX","6000","Fansite\"","100020"]}`)
	h.post(t, `{"name":"b","indexer":"bitporn","categories":["XXX","6000","Compilation\"","100015"]}`)

	families, err := h.reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	got := map[string]float64{}
	var announces float64
	for _, f := range families {
		switch f.GetName() {
		case "stashtrap_announce_categories_total":
			for _, m := range f.GetMetric() {
				var id, name string
				for _, l := range m.GetLabel() {
					switch l.GetName() {
					case "category_id":
						id = l.GetValue()
					case "category":
						name = l.GetValue()
					}
				}
				got[id+"="+name] = m.GetCounter().GetValue()
			}
		case "stashtrap_announces_total":
			for _, m := range f.GetMetric() {
				announces += m.GetCounter().GetValue()
			}
		}
	}

	// The standard bucket rides on every announce; each site-specific one
	// appears once. The trailing quote Prowlarr's definition leaves on every
	// site-specific name is stripped before it becomes a label — the id is
	// the stable key, but the name is what a dashboard renders.
	want := map[string]float64{
		"6000=XXX":           2,
		"100020=Fansite":     1,
		"100015=Compilation": 1,
	}
	if len(got) != len(want) {
		t.Fatalf("category series = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("category %q = %v, want %v", k, got[k], v)
		}
	}

	// The point of a separate metric: announces are still counted once each.
	if announces != 2 {
		t.Errorf("stashtrap_announces_total = %v, want 2 despite 4 category assignments", announces)
	}
}

// An announce with no categories must not invent a series.
func TestAnnounceWithoutCategoriesRecordsNoCategoryMetric(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.post(t, `{"name":"a","indexer":"irc"}`)

	families, err := h.reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, f := range families {
		if f.GetName() == "stashtrap_announce_categories_total" && len(f.GetMetric()) != 0 {
			t.Errorf("category metric has %d series, want none", len(f.GetMetric()))
		}
	}
}
