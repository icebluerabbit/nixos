package tools

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"homelab/sys-mcp/internal/app"
	"homelab/sys-mcp/internal/cache"
	"homelab/sys-mcp/internal/config"
)

// obsStub serves both Prometheus and Loki from one address: the two clients
// hit disjoint paths, so a single mux stands in for both datasources.
func obsStub(t *testing.T) string {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(familyFixture))
	})
	mux.HandleFunc("/api/v1/label/__name__/values", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":["node_load1","qbittorrent_a"]}`))
	})
	mux.HandleFunc("/loki/api/v1/labels", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":["host","unit"]}`))
	})
	mux.HandleFunc("/loki/api/v1/label/host/values", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":["server","qbittorrent","loki-only"]}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

// obsServer registers the observability tools against the stub, with one
// alert rule and one dashboard so the cross-references have something to
// find.
func obsServer(t *testing.T, promURL, lokiURL string) *server.MCPServer {
	t.Helper()
	root := t.TempDir()
	ic := cache.NewInfraCacheWithData(testData)
	ac := &app.AppContext{
		Cfg: &config.Config{
			Hostname:      "desktop",
			FlakePath:     root,
			PrometheusURL: promURL,
			LokiURL:       lokiURL,
		},
		IC: ic,
		HC: cache.NewHostsCacheWithData(cache.HostsData{
			"server":  {Containers: map[string]cache.ContainerConf{"grafana": {}, "radarr": {}}},
			"desktop": {},
		}),
		SI: cache.NewSourceIndex(root, ic),
		AC: cache.NewAlertsCacheWithData([]cache.AlertRule{{
			UID:       "qbt-down",
			Title:     "QbittorrentDown",
			Group:     "qbittorrent",
			Category:  "qbittorrent",
			Expr:      "qbittorrent_a < 1",
			Op:        "lt",
			Threshold: 1,
			Severity:  "critical",
			For:       "5m",
			Metrics:   []string{"qbittorrent_a"},
		}}),
		DC: cache.NewDashboardCacheWithData([]cache.Dashboard{{
			Name:    "qbittorrent",
			Source:  "modules/hosts/server/containers/grafana/dashboards/_qbt.nix",
			Metrics: []string{"qbittorrent_a"},
		}}),
	}
	s := server.NewMCPServer("test", "0.0.0")
	RegisterObservabilityTools(s, ac)
	return s
}

func TestRegisterObservabilityToolsRegistersCatalog(t *testing.T) {
	url := obsStub(t)
	names := toolNames(t, obsServer(t, url, url))

	for _, want := range []string{
		"describe_observability", "get_metric", "list_metric_families",
		"list_alert_thresholds", "check_signal_coverage",
	} {
		if !has(names, want) {
			t.Errorf("tool %q is not registered; got %v", want, names)
		}
	}
}

// describe_observability is the orientation call: it reports what a target
// emits before anybody goes looking for evidence.
func TestDescribeObservabilityTool(t *testing.T) {
	url := obsStub(t)

	out, isErr := callTool(t, obsServer(t, url, url), "describe_observability",
		map[string]any{"target": "qbittorrent"})
	if isErr {
		t.Fatalf("describe_observability reported an error: %s", out)
	}
	if !strings.Contains(out, "qbittorrent") {
		t.Errorf("target missing from result: %s", out)
	}
}

func TestDescribeObservabilityToolRequiresTarget(t *testing.T) {
	url := obsStub(t)

	if _, isErr := callTool(t, obsServer(t, url, url), "describe_observability", map[string]any{}); !isErr {
		t.Error("missing target should be an error result")
	}
}

// get_metric is what stops a value being called healthy without consulting
// the threshold that already defines bad, so the rule must come back with it.
func TestGetMetricToolCarriesItsAlertRule(t *testing.T) {
	url := obsStub(t)

	out, isErr := callTool(t, obsServer(t, url, url), "get_metric",
		map[string]any{"name": "qbittorrent_a"})
	if isErr {
		t.Fatalf("get_metric reported an error: %s", out)
	}
	if !strings.Contains(out, "QbittorrentDown") {
		t.Errorf("alert rule missing from get_metric: %s", out)
	}
	if !strings.Contains(out, "qbt") {
		t.Errorf("dashboard missing from get_metric: %s", out)
	}
}

func TestGetMetricToolRequiresName(t *testing.T) {
	url := obsStub(t)

	if _, isErr := callTool(t, obsServer(t, url, url), "get_metric", map[string]any{}); !isErr {
		t.Error("missing name should be an error result")
	}
}

func TestListMetricFamiliesTool(t *testing.T) {
	url := obsStub(t)
	s := obsServer(t, url, url)

	out, isErr := callTool(t, s, "list_metric_families", nil)
	if isErr {
		t.Fatalf("list_metric_families reported an error: %s", out)
	}
	if !strings.Contains(out, "qbittorrent_") {
		t.Errorf("family missing: %s", out)
	}
	// Client-library boilerplate stays out of the orientation map.
	if strings.Contains(out, "go_goroutines") {
		t.Errorf("go_ boilerplate leaked into the family list: %s", out)
	}

	if out, isErr := callTool(t, s, "list_metric_families", map[string]any{"host": "server"}); isErr {
		t.Errorf("host-scoped list_metric_families reported an error: %s", out)
	}
}

func TestListAlertThresholdsTool(t *testing.T) {
	url := obsStub(t)
	s := obsServer(t, url, url)

	out, isErr := callTool(t, s, "list_alert_thresholds", nil)
	if isErr {
		t.Fatalf("list_alert_thresholds reported an error: %s", out)
	}
	if !strings.Contains(out, "QbittorrentDown") {
		t.Errorf("rule missing: %s", out)
	}

	// Filters narrow to a category and to the rules touching one metric.
	if out, isErr := callTool(t, s, "list_alert_thresholds",
		map[string]any{"category": "qbittorrent"}); isErr || !strings.Contains(out, "QbittorrentDown") {
		t.Errorf("category filter dropped the rule: %s", out)
	}
	if out, isErr := callTool(t, s, "list_alert_thresholds",
		map[string]any{"metric": "qbittorrent_a"}); isErr || !strings.Contains(out, "QbittorrentDown") {
		t.Errorf("metric filter dropped the rule: %s", out)
	}
}

// The gap report is the difference between "healthy" and "unmonitored": a
// host that only logs must show up as missing metrics.
func TestCheckSignalCoverageTool(t *testing.T) {
	url := obsStub(t)

	out, isErr := callTool(t, obsServer(t, url, url), "check_signal_coverage", nil)
	if isErr {
		t.Fatalf("check_signal_coverage reported an error: %s", out)
	}
	if !strings.Contains(out, "loki-only") {
		t.Errorf("log-only host missing from the gap report: %s", out)
	}
}

// An unreachable datasource is reported as such rather than as an empty
// estate, which would read as "nothing is wrong".
func TestObservabilityToolsReportUnreachableDatasource(t *testing.T) {
	s := obsServer(t, "http://127.0.0.1:1", "http://127.0.0.1:1")

	out, _ := callTool(t, s, "list_metric_families", nil)
	if !strings.Contains(strings.ToLower(out), "unreachable") &&
		!strings.Contains(strings.ToLower(out), "error") &&
		!strings.Contains(strings.ToLower(out), "refused") {
		t.Errorf("an unreachable Prometheus should be reported: %s", out)
	}
}
