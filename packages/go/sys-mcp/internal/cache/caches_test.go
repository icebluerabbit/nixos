package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The WithData constructors are how every tool test builds a cache without a
// flake on disk; a cache built that way answers from memory and never shells
// out to nix.
func TestCachesServePreloadedData(t *testing.T) {
	infra := NewInfraCacheWithData(&InfraData{
		Vlans: map[string]Vlan{"services": {VlanId: 140}},
	})
	got, err := infra.Get()
	if err != nil {
		t.Fatalf("InfraCache.Get: %v", err)
	}
	if got.Vlans["services"].VlanId != 140 {
		t.Errorf("infra data not served: %+v", got)
	}

	hosts := NewHostsCacheWithData(HostsData{"server": {}})
	hostData, err := hosts.Get()
	if err != nil {
		t.Fatalf("HostsCache.Get: %v", err)
	}
	if _, ok := hostData["server"]; !ok {
		t.Errorf("hosts data not served: %+v", hostData)
	}

	alerts := NewAlertsCacheWithData([]AlertRule{{Title: "Down"}})
	rules, err := alerts.Get()
	if err != nil {
		t.Fatalf("AlertsCache.Get: %v", err)
	}
	if len(rules) != 1 || rules[0].Title != "Down" {
		t.Errorf("alert data not served: %+v", rules)
	}

	boards := NewDashboardCacheWithData([]Dashboard{{Name: "qbittorrent"}})
	dashboards, err := boards.Get()
	if err != nil {
		t.Fatalf("DashboardCache.Get: %v", err)
	}
	if len(dashboards) != 1 || dashboards[0].Name != "qbittorrent" {
		t.Errorf("dashboard data not served: %+v", dashboards)
	}
}

// A cache with neither data nor a flake path has nothing to answer with, and
// says so rather than returning an empty estate.
func TestCachesReportMissingSource(t *testing.T) {
	if _, err := (&AlertsCache{}).Get(); err == nil {
		t.Error("AlertsCache.Get succeeded with no data and no flake path")
	}
}

// Prewarm is fire-and-forget: it must return immediately and must not panic
// when the evaluation behind it fails, which is what happens on a directory
// that is not a flake.
func TestPrewarmIsNonBlockingAndSafe(t *testing.T) {
	dir := t.TempDir()

	NewInfraCache(dir).Prewarm()
	NewHostsCache(dir).Prewarm()
	NewAlertsCache(dir).Prewarm()

	// Give the goroutines a moment to fail; the test is that nothing panics
	// and the process stays healthy.
	time.Sleep(50 * time.Millisecond)
}

// Evaluation failures surface as errors on the first call, when there is no
// previous result to fall back on.
func TestCachesReportEvalFailure(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{
		filepath.Join("modules", "infra"),
		filepath.Join("modules", "hosts"),
		filepath.Join("modules", "containers"),
		filepath.Join("modules", "hosts", "server", "containers", "grafana"),
	} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := NewInfraCache(dir).Get(); err == nil {
		t.Error("InfraCache.Get succeeded against a directory that is not a flake")
	}
	if _, err := NewHostsCache(dir).Get(); err == nil {
		t.Error("HostsCache.Get succeeded against a directory that is not a flake")
	}
	if _, err := NewAlertsCache(dir).Get(); err == nil {
		t.Error("AlertsCache.Get succeeded against a directory that is not a flake")
	}
}

// The alerts cache keys its freshness off the grafana container directory, so
// a missing one is named rather than reported as an empty rule set.
func TestAlertsCacheReportsMissingGrafanaDir(t *testing.T) {
	_, err := NewAlertsCache(t.TempDir()).Get()

	if err == nil || !strings.Contains(err.Error(), "grafana dir not found") {
		t.Errorf("error = %v, want a missing grafana directory", err)
	}
}

// A cache that already holds data keeps serving it when the evaluation fails:
// a stale answer beats no answer for a tool that resolves where to look.
func TestCachesKeepStaleDataOnEvalFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "modules", "infra"), 0o755); err != nil {
		t.Fatal(err)
	}

	c := NewInfraCache(dir)
	c.data = &InfraData{Vlans: map[string]Vlan{"stale": {VlanId: 10}}}

	got, err := c.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, ok := got.Vlans["stale"]; !ok {
		t.Errorf("previous data was discarded on failure: %+v", got)
	}
}
