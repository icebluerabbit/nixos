package cache

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// AlertRule is one provisioned Grafana rule, flattened.
//
// Read from evaluated config rather than the alert aspect sources: the rules
// are assembled through _lib.nix builders, so the source files do not carry
// the final expression, and regexing a threshold out of them is wrong often
// enough to mislead. The evaluated form has the exact number.
type AlertRule struct {
	UID           string  `json:"uid"`
	Title         string  `json:"title"`
	Group         string  `json:"group"`
	Expr          string  `json:"expr"`
	Op            string  `json:"op"`
	Threshold     float64 `json:"threshold"`
	For           string  `json:"for"`
	Severity      string  `json:"severity"`
	Category      string  `json:"category"`
	NoDataState   string  `json:"noDataState"`
	Summary       string  `json:"summary"`
	Description   string  `json:"description"`
	DatasourceUID string  `json:"datasourceUid"`
	// Metrics are the metric names referenced by Expr, so a lookup can go
	// from a metric to the rule that governs it.
	Metrics []string `json:"metrics"`
}

// rawGroup mirrors the provisioned alerting JSON closely enough to flatten.
type rawGroup struct {
	Name  string `json:"name"`
	Rules []struct {
		UID         string            `json:"uid"`
		Title       string            `json:"title"`
		For         string            `json:"for"`
		NoDataState string            `json:"noDataState"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
		Data        []struct {
			DatasourceUID string `json:"datasourceUid"`
			Model         struct {
				Expr       string `json:"expr"`
				Conditions []struct {
					Evaluator struct {
						Params []float64 `json:"params"`
						Type   string    `json:"type"`
					} `json:"evaluator"`
				} `json:"conditions"`
			} `json:"model"`
		} `json:"data"`
	} `json:"rules"`
}

const alertsAttr = "#nixosConfigurations.server.config.containers.grafana" +
	".config.services.grafana.provision.alerting.rules.settings.groups"

// AlertsCache evaluates and caches the provisioned Grafana alert rules.
//
// Same mtime-gated shape as InfraCache, watching the grafana aspect tree.
// A failed eval falls back to the last good data rather than erroring: a
// broken rule file should not take out every observability tool.
type AlertsCache struct {
	mu        sync.RWMutex
	data      []AlertRule
	lastMtime time.Time
	flakePath string
}

func NewAlertsCache(flakePath string) *AlertsCache {
	return &AlertsCache{flakePath: flakePath}
}

// NewAlertsCacheWithData returns a pre-loaded cache for use in tests.
func NewAlertsCacheWithData(rules []AlertRule) *AlertsCache {
	return &AlertsCache{data: rules}
}

// Prewarm kicks off an async eval so the first tool call does not block.
func (c *AlertsCache) Prewarm() {
	go func() { _, _ = c.refresh() }()
}

func (c *AlertsCache) Get() ([]AlertRule, error) {
	if c.flakePath == "" {
		c.mu.RLock()
		data := c.data
		c.mu.RUnlock()
		if data != nil {
			return data, nil
		}
		return nil, fmt.Errorf("no data and no flakePath")
	}

	mtime, err := c.latestMtime()
	if err != nil {
		return nil, fmt.Errorf("mtime check: %w", err)
	}

	c.mu.RLock()
	if c.data != nil && !mtime.After(c.lastMtime) {
		data := c.data
		c.mu.RUnlock()
		return data, nil
	}
	c.mu.RUnlock()

	return c.refresh()
}

func (c *AlertsCache) refresh() ([]AlertRule, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	out, err := exec.Command(
		"nix", "eval", c.flakePath+alertsAttr,
		"--json", "--no-write-lock-file",
	).Output()
	if err != nil {
		if c.data != nil {
			return c.data, nil
		}
		return nil, fmt.Errorf("nix eval failed: %w", err)
	}

	var groups []rawGroup
	if err := json.Unmarshal(out, &groups); err != nil {
		if c.data != nil {
			return c.data, nil
		}
		return nil, fmt.Errorf("parse alert json: %w", err)
	}

	rules := FlattenAlertGroups(groups)
	mtime, _ := c.latestMtime()
	c.data = rules
	c.lastMtime = mtime
	return c.data, nil
}

// FlattenAlertGroups converts the provisioned shape into flat rules.
// Exported so tests can exercise it against a fixture.
func FlattenAlertGroups(groups []rawGroup) []AlertRule {
	var rules []AlertRule
	for _, group := range groups {
		for _, r := range group.Rules {
			rule := AlertRule{
				UID:         r.UID,
				Title:       r.Title,
				Group:       group.Name,
				For:         r.For,
				NoDataState: r.NoDataState,
				Severity:    r.Labels["severity"],
				Category:    r.Labels["category"],
				Summary:     r.Annotations["summary"],
				Description: r.Annotations["description"],
			}
			// A rule is two queries: A fetches from a real datasource,
			// B applies the __expr__ threshold that makes it boolean.
			for _, d := range r.Data {
				if d.Model.Expr != "" && rule.Expr == "" {
					rule.Expr = d.Model.Expr
					rule.DatasourceUID = d.DatasourceUID
				}
				for _, cond := range d.Model.Conditions {
					if len(cond.Evaluator.Params) > 0 && rule.Op == "" {
						rule.Op = cond.Evaluator.Type
						rule.Threshold = cond.Evaluator.Params[0]
					}
				}
			}
			rule.Metrics = MetricNamesIn(rule.Expr)
			rules = append(rules, rule)
		}
	}
	return rules
}

func (c *AlertsCache) latestMtime() (time.Time, error) {
	var latest time.Time
	dir := filepath.Join(
		c.flakePath, "modules", "hosts", "server", "containers", "grafana",
	)
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if t := info.ModTime(); t.After(latest) {
			latest = t
		}
		return nil
	})
	if err != nil && os.IsNotExist(err) {
		return time.Time{}, fmt.Errorf("grafana dir not found: %s", dir)
	}
	return latest, err
}
