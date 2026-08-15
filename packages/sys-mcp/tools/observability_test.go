package tools

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sys-mcp/cache"
)

// promStub serves a fixed instant-vector response for any query.
func promStub(t *testing.T, body string) *cache.Datasource {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		},
	))
	t.Cleanup(srv.Close)
	return cache.NewDatasource(srv.URL, srv.URL, 5*time.Second)
}

const familyFixture = `{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {"metric": {"__name__": "qbittorrent_a", "host": "qbittorrent"},
       "value": [1, "10"]},
      {"metric": {"__name__": "qbittorrent_b", "host": "qbittorrent"},
       "value": [1, "5"]},
      {"metric": {"__name__": "node_load1", "host": "server"},
       "value": [1, "2"]},
      {"metric": {"__name__": "go_goroutines", "host": "server"},
       "value": [1, "99"]}
    ]
  }
}`

func TestFamiliesForGroupsAndRanks(t *testing.T) {
	families, err := familiesFor(promStub(t, familyFixture), `{}`)
	if err != nil {
		t.Fatal(err)
	}

	// go_ is client-library boilerplate and must not appear even though
	// it carries the highest series count in the fixture.
	if len(families) != 2 {
		t.Fatalf("got %d families, want 2: %+v", len(families), families)
	}
	if families[0].Prefix != "qbittorrent_" {
		t.Errorf("first family = %q, want qbittorrent_", families[0].Prefix)
	}
	if families[0].Series != 15 {
		t.Errorf("qbittorrent_ series = %d, want 15", families[0].Series)
	}
	if len(families[0].Hosts) != 1 || families[0].Hosts[0] != "qbittorrent" {
		t.Errorf("hosts = %v", families[0].Hosts)
	}
	for _, f := range families {
		if f.Prefix == "go_" {
			t.Error("boilerplate prefix go_ should be filtered out")
		}
	}
}

// A datasource that cannot be reached must degrade to data, not an error:
// sys-mcp is what you reach for when Prometheus itself is the problem.
func TestUnreachableIsDataNotFailure(t *testing.T) {
	ds := cache.NewDatasource("http://127.0.0.1:1", "", time.Millisecond)
	_, err := ds.Query("up")
	if err == nil {
		t.Fatal("expected the query to fail")
	}

	got := unreachable(err)
	if got["reporting"] != false {
		t.Error("unreachable must not report as healthy")
	}
	if got["error"] == "" || got["note"] == "" {
		t.Error("unreachable must explain itself")
	}
}

// An unset address is a provisioning gap in mcp.nix; a dead datasource is an
// incident. They must not produce the same advice.
func TestUnsetAddressIsDistinguishedFromOutage(t *testing.T) {
	unset := cache.NewDatasource("", "", time.Second)

	_, err := unset.Query("up")
	if !errors.Is(err, cache.ErrUnsetPrometheus) {
		t.Fatalf("Query err = %v, want ErrUnsetPrometheus", err)
	}
	if _, err := unset.LokiLabels(); !errors.Is(err, cache.ErrUnsetLoki) {
		t.Fatalf("LokiLabels err = %v, want ErrUnsetLoki", err)
	}

	unsetNote := unreachable(cache.ErrUnsetPrometheus)["note"].(string)
	if !strings.Contains(unsetNote, "mcp.nix") {
		t.Errorf("unset note should point at mcp.nix, got %q", unsetNote)
	}

	down := cache.NewDatasource("http://127.0.0.1:1", "", time.Millisecond)
	_, derr := down.Query("up")
	if derr == nil {
		t.Fatal("expected a connection failure")
	}
	downNote := unreachable(derr)["note"].(string)
	if strings.Contains(downNote, "mcp.nix") {
		t.Errorf("an outage must not be reported as misconfiguration: %q",
			downNote)
	}
}
