package cache

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Dashboard is one Grafana dashboard aspect and the metrics it charts.
type Dashboard struct {
	Name    string   `json:"name"`
	Source  string   `json:"source"`
	Metrics []string `json:"metrics"`
}

// DashboardCache indexes the dashboard aspects by the metrics they chart.
//
// Scans source rather than evaluating: the panel `description` fields are the
// best available definition of what each metric means in this estate, and
// they are easier to read in the aspect than in provisioned JSON. Tools hand
// back the source path so the description can be read in place instead of
// being copied out and going stale.
type DashboardCache struct {
	mu        sync.RWMutex
	data      []Dashboard
	lastMtime time.Time
	dir       string
}

func NewDashboardCache(flakePath string) *DashboardCache {
	return &DashboardCache{dir: filepath.Join(
		flakePath, "modules", "hosts", "server",
		"containers", "grafana", "dashboards",
	)}
}

// NewDashboardCacheWithData returns a pre-loaded cache for use in tests.
func NewDashboardCacheWithData(boards []Dashboard) *DashboardCache {
	return &DashboardCache{data: boards}
}

func (c *DashboardCache) Get() ([]Dashboard, error) {
	if c.dir == "" {
		c.mu.RLock()
		defer c.mu.RUnlock()
		return c.data, nil
	}

	mtime := c.latestMtime()
	c.mu.RLock()
	if c.data != nil && !mtime.After(c.lastMtime) {
		data := c.data
		c.mu.RUnlock()
		return data, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	paths, err := filepath.Glob(filepath.Join(c.dir, "*.nix"))
	if err != nil {
		return c.data, err
	}
	sort.Strings(paths)

	var boards []Dashboard
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".nix")
		// _panels.nix and _lib-style helpers define shared builders, not
		// a dashboard. Indexing them would attribute every metric they
		// mention to a dashboard nobody can open.
		if name == "_panels" {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		boards = append(boards, Dashboard{
			Name:    strings.TrimPrefix(name, "_"),
			Source:  path,
			Metrics: MetricTokens(string(raw)),
		})
	}

	c.data = boards
	c.lastMtime = mtime
	return c.data, nil
}

// MetricsFor returns the dashboards charting a metric.
func (c *DashboardCache) MetricsFor(metric string) []Dashboard {
	boards, err := c.Get()
	if err != nil {
		return nil
	}
	var out []Dashboard
	for _, board := range boards {
		for _, m := range board.Metrics {
			if m == metric {
				out = append(out, board)
				break
			}
		}
	}
	return out
}

func (c *DashboardCache) latestMtime() time.Time {
	var latest time.Time
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return latest
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if t := info.ModTime(); t.After(latest) {
			latest = t
		}
	}
	return latest
}
