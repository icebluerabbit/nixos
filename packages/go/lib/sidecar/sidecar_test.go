package sidecar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"homelab/app"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fake is a Service under test control.
type fake struct {
	info    Info
	changes func(ctx context.Context, cursor string, limit int) ([]Event, error)
	enrich  func(ctx context.Context, body []byte) ([]Event, error)
	routes  func(mux *http.ServeMux)
}

func (f *fake) Info() Info { return f.info }

func (f *fake) Changes(ctx context.Context, cursor string, limit int) ([]Event, error) {
	if f.changes == nil {
		return nil, ErrUnsupported
	}
	return f.changes(ctx, cursor, limit)
}

func (f *fake) Enrich(ctx context.Context, body []byte) ([]Event, error) {
	if f.enrich == nil {
		return nil, ErrUnsupported
	}
	return f.enrich(ctx, body)
}

// extended adds Extender to fake. Separate type because a Service must be able
// to NOT implement Extender — that is what the optional half means.
type extended struct{ *fake }

func (e extended) Routes(mux *http.ServeMux) { e.routes(mux) }

func newHandler(t *testing.T, svc Service, relay *Relay) (http.Handler, *Metrics) {
	t.Helper()
	m := NewMetrics(nil, "test")
	return Handler(discardLogger(), Defaults(), svc, relay, m), m
}

func do(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, http.NoBody)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	h.ServeHTTP(rec, r)
	return rec
}

func TestHealthz(t *testing.T) {
	h, _ := newHandler(t, &fake{}, nil)

	rec := do(t, h, http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Errorf("healthz = %d %q", rec.Code, rec.Body.String())
	}
}

// /healthz is liveness. A probe that depends on the upstream restarts this
// process for a problem restarting cannot fix, so it must answer even when
// every other route is failing.
func TestHealthzIgnoresBrokenService(t *testing.T) {
	svc := &fake{changes: func(context.Context, string, int) ([]Event, error) {
		return nil, errors.New("upstream on fire")
	}}
	h, _ := newHandler(t, svc, nil)

	if rec := do(t, h, http.MethodGet, "/healthz", ""); rec.Code != http.StatusOK {
		t.Errorf("healthz = %d, want 200 with a broken upstream", rec.Code)
	}
}

func TestInfo(t *testing.T) {
	want := Info{
		Sidecar:      "managarr-test",
		Source:       "test",
		Upstream:     "http://upstream:1234",
		Capabilities: []string{CapChanges, CapEvents},
	}
	h, _ := newHandler(t, &fake{info: want}, nil)

	rec := do(t, h, http.MethodGet, "/info", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got Info
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Sidecar != want.Sidecar || got.Source != want.Source ||
		got.Upstream != want.Upstream || len(got.Capabilities) != 2 {
		t.Errorf("info = %+v, want %+v", got, want)
	}
}

func TestChanges(t *testing.T) {
	var gotCursor string
	var gotLimit int
	svc := &fake{changes: func(_ context.Context, cursor string, limit int) ([]Event, error) {
		gotCursor, gotLimit = cursor, limit
		return []Event{{
			Source: "test", Kind: "thing", Ref: "abc", Cursor: "c1",
			OccurredAt: time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
		}}, nil
	}}
	h, _ := newHandler(t, svc, nil)

	rec := do(t, h, http.MethodGet, "/changes?since=c0&limit=7", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if gotCursor != "c0" || gotLimit != 7 {
		t.Errorf("service got cursor %q limit %d", gotCursor, gotLimit)
	}

	var body struct{ Events []Event }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Events) != 1 || body.Events[0].Cursor != "c1" {
		t.Errorf("events = %+v", body.Events)
	}
}

func TestChangesDefaults(t *testing.T) {
	var gotCursor string
	var gotLimit int
	svc := &fake{changes: func(_ context.Context, cursor string, limit int) ([]Event, error) {
		gotCursor, gotLimit = cursor, limit
		return nil, nil
	}}
	h, _ := newHandler(t, svc, nil)

	do(t, h, http.MethodGet, "/changes", "")
	if gotCursor != "" {
		t.Errorf("cursor = %q, want empty for the first call", gotCursor)
	}
	if gotLimit != defaultChangesLimit {
		t.Errorf("limit = %d, want %d", gotLimit, defaultChangesLimit)
	}
}

// The response is assembled in memory before any of it is sent, so an
// unbounded limit would let one request ask for an entire history. Clamped
// rather than refused — a consumer asking too much gets the maximum.
func TestChangesLimitClamped(t *testing.T) {
	var gotLimit int
	svc := &fake{changes: func(_ context.Context, _ string, limit int) ([]Event, error) {
		gotLimit = limit
		return nil, nil
	}}
	h, _ := newHandler(t, svc, nil)

	rec := do(t, h, http.MethodGet, fmt.Sprintf("/changes?limit=%d", maxChangesLimit*10), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want the limit clamped rather than refused", rec.Code)
	}
	if gotLimit != maxChangesLimit {
		t.Errorf("limit = %d, want %d", gotLimit, maxChangesLimit)
	}
}

func TestChangesBadLimit(t *testing.T) {
	h, _ := newHandler(t, &fake{changes: func(context.Context, string, int) ([]Event, error) {
		return nil, nil
	}}, nil)

	for _, target := range []string{"/changes?limit=abc", "/changes?limit=-1"} {
		if rec := do(t, h, http.MethodGet, target, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", target, rec.Code)
		}
	}
}

// Empty must be an array, never null: a walk that stops on an empty page has to
// tell "no more events" from "the field was missing".
func TestChangesEmptyIsAnArray(t *testing.T) {
	h, _ := newHandler(t, &fake{changes: func(context.Context, string, int) ([]Event, error) {
		return nil, nil
	}}, nil)

	rec := do(t, h, http.MethodGet, "/changes", "")
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if string(raw["events"]) != "[]" {
		t.Errorf("events = %s, want []", raw["events"])
	}
}

// 501, not 500. "This upstream keeps no history" is a fact about the service,
// not a fault, and a consumer should be able to stop asking rather than retry.
func TestChangesUnsupported(t *testing.T) {
	h, _ := newHandler(t, &fake{}, nil)

	if rec := do(t, h, http.MethodGet, "/changes", ""); rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", rec.Code)
	}
}

func TestChangesError(t *testing.T) {
	h, _ := newHandler(t, &fake{changes: func(context.Context, string, int) ([]Event, error) {
		return nil, errors.New("upstream refused")
	}}, nil)

	if rec := do(t, h, http.MethodGet, "/changes", ""); rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestEventsRelays(t *testing.T) {
	var got struct {
		Events []Event `json:"events"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	m := NewMetrics(nil, "test")
	relay, err := NewRelay(httpxConfig(upstream.URL, discardLogger()), "", discardLogger(), m)
	if err != nil {
		t.Fatal(err)
	}

	// One webhook, several events — a season pack import is one notification
	// and an event per file, and collapsing them would lose the detail.
	svc := &fake{enrich: func(_ context.Context, body []byte) ([]Event, error) {
		return []Event{
			{Source: "test", Kind: "import", Ref: string(body)},
			{Source: "test", Kind: "import", Ref: "second"},
		}, nil
	}}
	h := Handler(discardLogger(), Defaults(), svc, relay, m)

	rec := do(t, h, http.MethodPost, "/events", "payload")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if len(got.Events) != 2 || got.Events[0].Ref != "payload" {
		t.Errorf("relayed %+v", got.Events)
	}
	if n := testutil.ToFloat64(m.relayed.WithLabelValues("import")); n != 2 {
		t.Errorf("relayed metric = %v, want 2", n)
	}
}

// No events is normal, not an empty result: health pings and event kinds this
// sidecar ignores both land here.
func TestEventsNoneProduced(t *testing.T) {
	relayed := false
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		relayed = true
	}))
	defer upstream.Close()

	m := NewMetrics(nil, "test")
	relay, _ := NewRelay(httpxConfig(upstream.URL, discardLogger()), "", discardLogger(), m)
	svc := &fake{enrich: func(context.Context, []byte) ([]Event, error) { return nil, nil }}
	h := Handler(discardLogger(), Defaults(), svc, relay, m)

	if rec := do(t, h, http.MethodPost, "/events", "ping"); rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if relayed {
		t.Error("relayed an empty batch")
	}
}

// managarr being down must still answer the *arr 202. *arr does not retry and
// can do nothing with a failure; the reconcile is what recovers the event, and
// a 500 would only make the upstream log an error about something handled.
func TestEventsRelayFailureStillAccepts(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	m := NewMetrics(nil, "test")
	cfg := httpxConfig(upstream.URL, discardLogger())
	cfg.RetryMax = -1 // no backoff in a test
	relay, _ := NewRelay(cfg, "", discardLogger(), m)

	svc := &fake{enrich: func(context.Context, []byte) ([]Event, error) {
		return []Event{{Source: "test", Kind: "grab"}}, nil
	}}
	h := Handler(discardLogger(), Defaults(), svc, relay, m)

	if rec := do(t, h, http.MethodPost, "/events", "x"); rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", rec.Code)
	}
	if n := testutil.ToFloat64(m.failed.WithLabelValues(StageRelay)); n != 1 {
		t.Errorf("failed[relay] = %v, want 1", n)
	}
}

// A sidecar with no managarr URL looks healthy and silently discards every
// webhook. It must at least count them, or the condition is invisible.
func TestEventsNoRelayCounts(t *testing.T) {
	svc := &fake{enrich: func(context.Context, []byte) ([]Event, error) {
		return []Event{{Source: "test", Kind: "grab"}}, nil
	}}
	h, m := newHandler(t, svc, nil)

	if rec := do(t, h, http.MethodPost, "/events", "x"); rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", rec.Code)
	}
	if n := testutil.ToFloat64(m.failed.WithLabelValues(StageRelay)); n != 1 {
		t.Errorf("failed[relay] = %v, want 1", n)
	}
}

func TestEventsUnsupported(t *testing.T) {
	h, _ := newHandler(t, &fake{}, nil)

	if rec := do(t, h, http.MethodPost, "/events", "x"); rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", rec.Code)
	}
}

func TestEventsEnrichFailure(t *testing.T) {
	svc := &fake{enrich: func(context.Context, []byte) ([]Event, error) {
		return nil, errors.New("upstream would not answer")
	}}
	h, m := newHandler(t, svc, nil)

	if rec := do(t, h, http.MethodPost, "/events", "x"); rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if n := testutil.ToFloat64(m.failed.WithLabelValues(StageEnrich)); n != 1 {
		t.Errorf("failed[enrich] = %v, want 1", n)
	}
}

// A body past the cap is refused rather than read. The sidecar sits inside
// another service's container; an unbounded read there is worth not having.
func TestEventsBodyTooLarge(t *testing.T) {
	cfg := Defaults()
	cfg.MaxBodyBytes = 16
	m := NewMetrics(nil, "test")
	svc := &fake{enrich: func(_ context.Context, body []byte) ([]Event, error) {
		return []Event{{Kind: "x", Ref: string(body)}}, nil
	}}
	h := Handler(discardLogger(), cfg, svc, nil, m)

	rec := do(t, h, http.MethodPost, "/events", strings.Repeat("a", 1024))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if n := testutil.ToFloat64(m.failed.WithLabelValues(StageDecode)); n != 1 {
		t.Errorf("failed[decode] = %v, want 1", n)
	}
}

// A zero cap means "use the default", not "refuse everything". Getting this
// backwards would make a sidecar that forgot the flag reject every webhook.
func TestZeroMaxBodyUsesDefault(t *testing.T) {
	cfg := Defaults()
	cfg.MaxBodyBytes = 0
	svc := &fake{enrich: func(context.Context, []byte) ([]Event, error) { return nil, nil }}
	h := Handler(discardLogger(), cfg, svc, nil, NewMetrics(nil, "test"))

	if rec := do(t, h, http.MethodPost, "/events", "a body"); rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestExtenderRoutes(t *testing.T) {
	svc := extended{&fake{routes: func(mux *http.ServeMux) {
		mux.HandleFunc("GET /client-torrents", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"clientTorrents":[]}`))
		})
	}}}
	h, _ := newHandler(t, svc, nil)

	rec := do(t, h, http.MethodGet, "/client-torrents", "")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want the extra route served", rec.Code)
	}
}

// A service that does not implement Extender must still serve the contract —
// the optional half has to be genuinely optional.
func TestServiceWithoutExtender(t *testing.T) {
	h, _ := newHandler(t, &fake{}, nil)

	if rec := do(t, h, http.MethodGet, "/healthz", ""); rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
}

// Extras are registered after the contract, so a service trying to take over
// /healthz or /changes panics rather than silently shadowing it.
func TestExtenderCannotShadowContract(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a service overriding a contract route must panic")
		}
	}()

	svc := extended{&fake{routes: func(mux *http.ServeMux) {
		mux.HandleFunc("GET /healthz", func(http.ResponseWriter, *http.Request) {})
	}}}
	newHandler(t, svc, nil)
}

func TestMethodsAndUnknownRoutes(t *testing.T) {
	h, _ := newHandler(t, &fake{}, nil)

	// The read routes take GET only; /events takes POST only.
	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/healthz"},
		{http.MethodPost, "/info"},
		{http.MethodPost, "/changes"},
		{http.MethodGet, "/events"},
		{http.MethodDelete, "/events"},
	} {
		if rec := do(t, h, c.method, c.path, ""); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d, want 405", c.method, c.path, rec.Code)
		}
	}
	if rec := do(t, h, http.MethodGet, "/admin", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown route = %d, want 404", rec.Code)
	}
}

// Every sidecar carries a source label, so one Grafana query can separate
// eight of them. A missing or wrong label makes the series unattributable.
func TestNewMetricsRegistersWithSourceLabel(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg, "cross-seed")

	// Counters with no observations are not gathered, so touch each series
	// first — otherwise this asserts nothing.
	m.received.WithLabelValues("decision").Inc()
	m.relayed.WithLabelValues("decision").Inc()
	m.failed.WithLabelValues(StageRelay).Inc()
	m.enrichDuration.WithLabelValues("ok").Observe(0.1)

	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]bool{}
	for _, f := range families {
		found[f.GetName()] = true
		for _, metric := range f.GetMetric() {
			var source string
			for _, l := range metric.GetLabel() {
				if l.GetName() == "source" {
					source = l.GetValue()
				}
			}
			if source != "cross-seed" {
				t.Errorf("%s has source=%q, want cross-seed", f.GetName(), source)
			}
		}
	}

	for _, name := range []string{
		"sidecar_events_received_total",
		"sidecar_events_relayed_total",
		"sidecar_events_failed_total",
		"sidecar_enrich_duration_seconds",
	} {
		if !found[name] {
			t.Errorf("%s was not registered", name)
		}
	}
}

// A nil registerer must still yield usable metrics: that is what lets a test
// build a handler without standing up a registry, and what keeps two sidecars
// in one process from colliding on registration.
func TestNewMetricsNilRegisterer(t *testing.T) {
	m := NewMetrics(nil, "test")

	m.received.WithLabelValues("x").Inc()
	if n := testutil.ToFloat64(m.received.WithLabelValues("x")); n != 1 {
		t.Errorf("counter = %v, want 1", n)
	}
}

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if cfg.Addr != DefaultAddr {
		t.Errorf("Addr = %q, want %q", cfg.Addr, DefaultAddr)
	}
	if cfg.MaxBodyBytes != defaultMaxBody {
		t.Errorf("MaxBodyBytes = %d", cfg.MaxBodyBytes)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v", cfg.ShutdownTimeout)
	}
}

// A sidecar that has received nothing must still export its event series.
//
// Without this, sidecar_events_* are absent until the first webhook — and
// "received zero" is then indistinguishable from "not scraped" and from "not
// deployed". An alert on events failing has nothing to fire against.
func TestRunRegistersEventSeriesForEventCapableServices(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg, "test")
	m.Register()

	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, f := range families {
		found[f.GetName()] = true
	}
	for _, name := range []string{
		"sidecar_events_received_total",
		"sidecar_events_failed_total",
	} {
		if !found[name] {
			t.Errorf("%s is absent before any event arrived", name)
		}
	}

	// Every failure stage, so a dashboard can distinguish which leg broke
	// rather than showing a gap for the two that have not fired.
	for _, stage := range []string{StageDecode, StageEnrich, StageRelay} {
		if got := testutil.ToFloat64(m.failed.WithLabelValues(stage)); got != 0 {
			t.Errorf("failed[%s] = %v, want a registered zero", stage, got)
		}
	}
}

// relayed stays absent on purpose: its label is the event kind, which a service
// invents as it goes. received rising while relayed is missing is the signal.
func TestRegisterLeavesRelayedAbsent(t *testing.T) {
	reg := prometheus.NewRegistry()
	NewMetrics(reg, "test").Register()

	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() == "sidecar_events_relayed_total" {
			t.Error("relayed was pre-registered; its kind label has no known set")
		}
	}
}

// A read-only sidecar must NOT publish event zeros: a flat zero on the relay
// counter would assert a push path that does not exist. cross-seed, bazarr,
// beets and slskd have no webhook at all.
func TestReadOnlyServiceGetsNoEventSeries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	reg := prometheus.NewRegistry()
	cfg := Defaults()
	cfg.Addr = "127.0.0.1:0"

	// Unsupported Enrich and no CapEvents — a reader, like managarr-crossseed.
	svc := &fake{info: Info{Source: "reader", Capabilities: []string{CapChanges}}}
	_ = Run(ctx, &app.Env{Logger: discardLogger(), Metrics: reg}, cfg, svc)

	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() == "sidecar_events_received_total" {
			t.Error("a read-only sidecar published an events counter")
		}
	}
}
