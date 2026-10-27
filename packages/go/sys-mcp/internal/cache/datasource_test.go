package cache

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// dsStub serves Prometheus and Loki from one address; the two clients use
// disjoint paths.
func dsStub(t *testing.T, handler http.Handler) *Datasource {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewDatasource(srv.URL+"/", srv.URL+"/", 5*time.Second)
}

// A trailing slash on either address must not produce a double slash in the
// request path; the addresses come from the environment and are written by
// hand in mcp.nix.
func TestNewDatasourceTrimsTrailingSlash(t *testing.T) {
	ds := NewDatasource("http://prom:9090/", "http://loki:3100/", time.Second)

	if strings.HasSuffix(ds.Prometheus, "/") || strings.HasSuffix(ds.Loki, "/") {
		t.Errorf("addresses keep their trailing slash: %q %q", ds.Prometheus, ds.Loki)
	}
	if ds.HTTP.Timeout != time.Second {
		t.Errorf("timeout = %v, want 1s", ds.HTTP.Timeout)
	}
}

func TestQueryReturnsSamples(t *testing.T) {
	var gotQuery string
	ds := dsStub(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"__name__":"node_load1","host":"server"},"value":[1,"0.5"]}]}}`))
	}))

	samples, err := ds.Query(`node_load1`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if gotQuery != "node_load1" {
		t.Errorf("expression sent = %q", gotQuery)
	}
	if len(samples) != 1 {
		t.Fatalf("got %d samples, want 1", len(samples))
	}
	if samples[0].Labels["host"] != "server" {
		t.Errorf("labels not parsed: %+v", samples[0])
	}
	if samples[0].Value != 0.5 {
		t.Errorf("value = %v, want 0.5", samples[0].Value)
	}
}

// An HTTP error is reported rather than read as an empty result set, which
// would look like a healthy estate with nothing wrong in it.
func TestQueryReportsHTTPFailure(t *testing.T) {
	ds := dsStub(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	if _, err := ds.Query(`up`); err == nil {
		t.Fatal("Query succeeded against a failing Prometheus")
	}
}

func TestQueryReportsUnreachable(t *testing.T) {
	ds := NewDatasource("http://127.0.0.1:1", "http://127.0.0.1:1", time.Second)

	_, err := ds.Query(`up`)
	if err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Errorf("error = %v, want an unreachable report", err)
	}
}

func TestMetricNames(t *testing.T) {
	ds := dsStub(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":["node_load1","up"]}`))
	}))

	names, err := ds.MetricNames()
	if err != nil {
		t.Fatalf("MetricNames: %v", err)
	}
	if len(names) != 2 || names[0] != "node_load1" {
		t.Errorf("names = %v", names)
	}
}

func TestLokiLabelsAndValues(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/loki/api/v1/labels", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":["host","unit"]}`))
	})
	mux.HandleFunc("/loki/api/v1/label/host/values", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":["server","qbittorrent"]}`))
	})
	ds := dsStub(t, mux)

	labels, err := ds.LokiLabels()
	if err != nil {
		t.Fatalf("LokiLabels: %v", err)
	}
	if len(labels) != 2 {
		t.Errorf("labels = %v", labels)
	}

	// Values come back sorted, so the caller can present them directly.
	values, err := ds.LokiValues("host")
	if err != nil {
		t.Fatalf("LokiValues: %v", err)
	}
	if len(values) != 2 || values[0] != "qbittorrent" || values[1] != "server" {
		t.Errorf("values = %v, want [qbittorrent server]", values)
	}
}

// A label name reaches the path escaped, so a label with a slash cannot
// rewrite the request.
func TestLokiValuesEscapesLabel(t *testing.T) {
	var gotPath string
	ds := dsStub(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":[]}`))
	}))

	if _, err := ds.LokiValues("a/b"); err != nil {
		t.Fatalf("LokiValues: %v", err)
	}
	if strings.Contains(gotPath, "a/b") {
		t.Errorf("label was not escaped into the path: %s", gotPath)
	}
}
