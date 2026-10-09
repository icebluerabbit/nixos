package cache

import (
	"os"
	"path/filepath"
	"testing"
)

// A dashboard aspect is nested nix attrsets, so every metric reference sits
// inside braces. Scanning it with the PromQL label-matcher stripper ate the
// whole file and silently reported that nothing was charted — the regression
// this guards.
const dashboardFixture = `{ lib, ... }:
{
  panels = [
    (timeseries {
      title = "Disk Queue";
      targets = [
        (target "qbittorrent_global_queued_io_jobs{${host}}" "Queued" "A")
        (target "qbittorrent_global_average_time_queue{${host}}" "Avg" "B")
      ];
      description = "write-path backlog waiting to flush to NFS";
    })
  ];
}`

func writeDashboard(t *testing.T, name, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(
		root, "modules", "hosts", "server", "containers",
		"grafana", "dashboards",
	)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDashboardCacheFindsMetricsInsideNixBraces(t *testing.T) {
	root := writeDashboard(t, "_qbittorrent.nix", dashboardFixture)

	boards, err := NewDashboardCache(root).Get()
	if err != nil {
		t.Fatal(err)
	}
	if len(boards) != 1 {
		t.Fatalf("got %d dashboards, want 1", len(boards))
	}
	if boards[0].Name != "qbittorrent" {
		t.Errorf("name = %q, want qbittorrent", boards[0].Name)
	}

	for _, want := range []string{
		"qbittorrent_global_queued_io_jobs",
		"qbittorrent_global_average_time_queue",
	} {
		found := false
		for _, metric := range boards[0].Metrics {
			if metric == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("dashboard scan missed %s", want)
		}
	}
}

func TestDashboardCacheMetricsForMatches(t *testing.T) {
	root := writeDashboard(t, "_qbittorrent.nix", dashboardFixture)
	dc := NewDashboardCache(root)

	hits := dc.MetricsFor("qbittorrent_global_average_time_queue")
	if len(hits) != 1 || hits[0].Name != "qbittorrent" {
		t.Fatalf("MetricsFor returned %+v", hits)
	}
	if hits[0].Source == "" {
		t.Error("source path must be returned so the panel description " +
			"can be read in place")
	}
	if got := dc.MetricsFor("nonexistent_metric_name"); len(got) != 0 {
		t.Errorf("expected no hits, got %+v", got)
	}
}

// _panels.nix defines shared panel builders, not a dashboard. Indexing it
// would attribute metrics to something nobody can open in Grafana.
func TestDashboardCacheSkipsPanelHelpers(t *testing.T) {
	root := writeDashboard(t, "_panels.nix", dashboardFixture)
	boards, err := NewDashboardCache(root).Get()
	if err != nil {
		t.Fatal(err)
	}
	if len(boards) != 0 {
		t.Errorf("_panels.nix should not register as a dashboard: %+v",
			boards)
	}
}
