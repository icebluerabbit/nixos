package tools

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"sys-mcp/app"
	"sys-mcp/cache"
)

// boilerplatePrefixes are emitted by every Go or Python service through its
// client library. They exist nearly everywhere and say nothing about this
// estate, so family listings summarise them instead of enumerating them.
var boilerplatePrefixes = map[string]bool{
	"deprecated":   true,
	"go":           true,
	"namedprocess": true,
	"process":      true,
	"promhttp":     true,
	"python":       true,
}

func RegisterObservabilityTools(s *server.MCPServer, ac *app.AppContext) {
	registerDescribeObservability(s, ac)
	registerGetMetric(s, ac)
	registerListMetricFamilies(s, ac)
	registerListAlertThresholds(s, ac)
	registerCheckSignalCoverage(s, ac)
}

func registerDescribeObservability(s *server.MCPServer, ac *app.AppContext) {
	s.AddTool(
		mcp.NewTool("describe_observability",
			mcp.WithDescription(
				"What signal a host or container actually emits: metric "+
					"families, whether it reaches Loki, the alert rules "+
					"governing it (with exact thresholds) and the "+
					"dashboards charting it. Start here before "+
					"investigating or tuning anything — it says what "+
					"evidence exists before you go looking for it.",
			),
			mcp.WithString("target",
				mcp.Required(),
				mcp.Description(
					"Host or container name, e.g. qbittorrent, server.",
				),
			),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			target, err := req.RequireString("target")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			ds := ac.Datasource()
			result := map[string]any{"target": target}

			selector := fmt.Sprintf(`{host=%q}`, target)
			families, ferr := familiesFor(ds, selector)
			if ferr != nil {
				result["metrics"] = unreachable(ferr)
			} else if len(families) == 0 {
				result["metrics"] = map[string]any{
					"reporting": false,
					"note": "no series carry host=" + target +
						"; it may log without emitting metrics — see " +
						"check_signal_coverage",
				}
			} else {
				result["metrics"] = map[string]any{
					"reporting": true,
					"families":  families,
				}
			}

			if hosts, lerr := ds.LokiValues("host"); lerr != nil {
				result["logs"] = unreachable(lerr)
			} else {
				result["logs"] = map[string]any{
					"reporting": contains(hosts, target),
					"query":     fmt.Sprintf(`{host=%q}`, target),
				}
			}

			result["alerts"] = alertsForHost(ac, target)
			result["dashboards"] = dashboardsForHost(ac, target)
			return jsonResult(result)
		},
	)
}

func registerGetMetric(s *server.MCPServer, ac *app.AppContext) {
	s.AddTool(
		mcp.NewTool("get_metric",
			mcp.WithDescription(
				"Everything known about one metric: which hosts report "+
					"it, its current value per host, the alert rule "+
					"governing it with the exact threshold and duration, "+
					"and the dashboards charting it. Call this before "+
					"describing any value as high, low, or healthy — the "+
					"threshold that defines bad is usually already agreed.",
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Exact metric name."),
			),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := req.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			ds := ac.Datasource()
			result := map[string]any{"metric": name}

			samples, qerr := ds.Query(name)
			switch {
			case qerr != nil:
				result["current"] = unreachable(qerr)
			case len(samples) == 0:
				result["current"] = map[string]any{
					"reporting": false,
					"note": "no live series — the name may still be in " +
						"the index while its scrape target is gone. A " +
						"query returns empty, not an error.",
				}
			default:
				byHost := map[string]float64{}
				for _, sample := range samples {
					host := sample.Labels["host"]
					if host == "" {
						host = "(unlabelled)"
					}
					// Several series per host is normal — one per
					// torrent, per device, per entity. Report the count
					// and the sum rather than pretending it is scalar.
					byHost[host] += sample.Value
				}
				result["current"] = map[string]any{
					"reporting":  true,
					"series":     len(samples),
					"sumPerHost": byHost,
					"note": "sums across every series carrying that host; " +
						"query Prometheus directly for per-label detail",
				}
			}

			result["governedBy"] = alertsForMetric(ac, name)
			result["chartedBy"] = dashboardsForMetric(ac, name)
			if len(result["governedBy"].([]map[string]any)) == 0 {
				result["judgement"] = "No committed threshold. Any claim " +
					"that a value is good or bad here is yours — say so."
			}
			return jsonResult(result)
		},
	)
}

func registerListMetricFamilies(s *server.MCPServer, ac *app.AppContext) {
	s.AddTool(
		mcp.NewTool("list_metric_families",
			mcp.WithDescription(
				"Metric name prefixes that exist, with series counts and "+
					"which hosts report them. The orientation map when you "+
					"do not yet know what is instrumented.",
			),
			mcp.WithString("host",
				mcp.Description(
					"Restrict to one host or container. Omit for the "+
						"whole estate.",
				),
			),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			host := req.GetString("host", "")
			selector := `{__name__=~".+"}`
			if host != "" {
				selector = fmt.Sprintf(`{host=%q}`, host)
			}
			families, err := familiesFor(ac.Datasource(), selector)
			if err != nil {
				return jsonResult(unreachable(err))
			}
			return jsonResult(map[string]any{
				"host":     host,
				"families": families,
			})
		},
	)
}

func registerListAlertThresholds(s *server.MCPServer, ac *app.AppContext) {
	s.AddTool(
		mcp.NewTool("list_alert_thresholds",
			mcp.WithDescription(
				"Every provisioned alert rule with its expression, exact "+
					"threshold, comparison, duration and severity. This is "+
					"what this homelab has already agreed counts as bad — "+
					"consult it instead of inventing a threshold.",
			),
			mcp.WithString("category",
				mcp.Description(
					"Filter by category label, e.g. qbittorrent, network.",
				),
			),
			mcp.WithString("metric",
				mcp.Description("Filter to rules referencing this metric."),
			),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			rules, err := ac.AC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			category := req.GetString("category", "")
			metric := req.GetString("metric", "")

			out := make([]cache.AlertRule, 0, len(rules))
			for _, rule := range rules {
				if category != "" && rule.Category != category {
					continue
				}
				if metric != "" && !contains(rule.Metrics, metric) {
					continue
				}
				out = append(out, rule)
			}
			return jsonResult(map[string]any{
				"count": len(out),
				"rules": out,
			})
		},
	)
}

func registerCheckSignalCoverage(s *server.MCPServer, ac *app.AppContext) {
	s.AddTool(
		mcp.NewTool("check_signal_coverage",
			mcp.WithDescription(
				"Estate-wide observability gaps: hosts that log but emit "+
					"no metrics, hosts that emit metrics but never log, and "+
					"metric families whose scrape target has gone away. "+
					"Run this when a query returns empty and you need to "+
					"know whether that means healthy or unmonitored.",
			),
		),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ds := ac.Datasource()
			result := map[string]any{}

			promHosts, perr := hostSet(ds)
			lokiHosts, lerr := ds.LokiValues("host")
			if perr != nil || lerr != nil {
				return jsonResult(map[string]any{
					"prometheus": errString(perr),
					"loki":       errString(lerr),
				})
			}

			var logsOnly, metricsOnly []string
			for _, host := range lokiHosts {
				if !contains(promHosts, host) {
					logsOnly = append(logsOnly, host)
				}
			}
			for _, host := range promHosts {
				if !contains(lokiHosts, host) {
					metricsOnly = append(metricsOnly, host)
				}
			}
			sort.Strings(logsOnly)
			sort.Strings(metricsOnly)

			result["logsButNoMetrics"] = map[string]any{
				"hosts": logsOnly,
				"note": "joining these on `host` in PromQL returns an " +
					"empty result, not an error. UniFi gear logs under " +
					"its device name while its metrics arrive via the " +
					"unpoller exporter under a different host.",
			}
			result["metricsButNoLogs"] = metricsOnly

			if stale, err := staleFamilies(ds); err == nil {
				result["staleFamilies"] = map[string]any{
					"prefixes": stale,
					"note": "names Prometheus still indexes with no live " +
						"series behind them — a dead exporter reads " +
						"identical to a healthy silent one",
				}
			}

			if services, err := ds.LokiValues("service_name"); err == nil {
				if len(services) <= 1 {
					result["serviceNameLabel"] = "unpopulated — nothing " +
						"sets it, so it cannot select a stream. Use `host`."
				}
			}
			return jsonResult(result)
		},
	)
}

// familiesFor groups the metric names under a selector by name prefix.
type family struct {
	Prefix string   `json:"prefix"`
	Series int      `json:"series"`
	Hosts  []string `json:"hosts,omitempty"`
}

func familiesFor(ds *cache.Datasource, selector string) ([]family, error) {
	samples, err := ds.Query(
		fmt.Sprintf("count by (__name__, host) (%s)", selector),
	)
	if err != nil {
		return nil, err
	}

	type acc struct {
		series int
		hosts  map[string]bool
	}
	grouped := map[string]*acc{}
	for _, sample := range samples {
		name := sample.Labels["__name__"]
		prefix, _, ok := strings.Cut(name, "_")
		if !ok || prefix == "" {
			continue
		}
		entry, seen := grouped[prefix]
		if !seen {
			entry = &acc{hosts: map[string]bool{}}
			grouped[prefix] = entry
		}
		entry.series += int(sample.Value)
		if host := sample.Labels["host"]; host != "" {
			entry.hosts[host] = true
		}
	}

	out := make([]family, 0, len(grouped))
	for prefix, entry := range grouped {
		if boilerplatePrefixes[prefix] {
			continue
		}
		hosts := make([]string, 0, len(entry.hosts))
		for host := range entry.hosts {
			hosts = append(hosts, host)
		}
		sort.Strings(hosts)
		out = append(out, family{
			Prefix: prefix + "_",
			Series: entry.series,
			Hosts:  hosts,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Series != out[j].Series {
			return out[i].Series > out[j].Series
		}
		return out[i].Prefix < out[j].Prefix
	})
	return out, nil
}

func staleFamilies(ds *cache.Datasource) ([]string, error) {
	names, err := ds.MetricNames()
	if err != nil {
		return nil, err
	}
	live, err := familiesFor(ds, `{__name__=~".+"}`)
	if err != nil {
		return nil, err
	}
	reporting := map[string]bool{}
	for _, f := range live {
		reporting[strings.TrimSuffix(f.Prefix, "_")] = true
	}

	seen := map[string]bool{}
	var stale []string
	for _, name := range names {
		prefix, _, ok := strings.Cut(name, "_")
		if !ok || reporting[prefix] || seen[prefix] {
			continue
		}
		if boilerplatePrefixes[prefix] {
			continue
		}
		seen[prefix] = true
		stale = append(stale, prefix+"_")
	}
	sort.Strings(stale)
	return stale, nil
}

func hostSet(ds *cache.Datasource) ([]string, error) {
	samples, err := ds.Query(`count by (host) ({__name__=~".+"})`)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(samples))
	for _, sample := range samples {
		if host := sample.Labels["host"]; host != "" {
			out = append(out, host)
		}
	}
	sort.Strings(out)
	return out, nil
}

func alertsForMetric(ac *app.AppContext, metric string) []map[string]any {
	out := []map[string]any{}
	rules, err := ac.AC.Get()
	if err != nil {
		return out
	}
	for _, rule := range rules {
		if !contains(rule.Metrics, metric) {
			continue
		}
		out = append(out, describeRule(rule))
	}
	return out
}

func alertsForHost(ac *app.AppContext, host string) []map[string]any {
	out := []map[string]any{}
	rules, err := ac.AC.Get()
	if err != nil {
		return out
	}
	needle := fmt.Sprintf("%q", host)
	for _, rule := range rules {
		// Host-scoped rules pin the host in a label matcher; fleet-wide
		// rules do not name one and apply to every reporting target.
		if rule.Category != host && !strings.Contains(rule.Expr, needle) {
			continue
		}
		out = append(out, describeRule(rule))
	}
	return out
}

func describeRule(rule cache.AlertRule) map[string]any {
	return map[string]any{
		"uid":         rule.UID,
		"title":       rule.Title,
		"fires":       fmt.Sprintf("%s %v", rule.Op, rule.Threshold),
		"for":         rule.For,
		"severity":    rule.Severity,
		"expr":        rule.Expr,
		"description": rule.Description,
	}
}

func dashboardsForMetric(ac *app.AppContext, metric string) []map[string]any {
	out := []map[string]any{}
	for _, board := range ac.DC.MetricsFor(metric) {
		out = append(out, map[string]any{
			"name":   board.Name,
			"source": board.Source,
			"note": "panel `description` fields in this aspect are the " +
				"best available definition of what the metric means",
		})
	}
	return out
}

func dashboardsForHost(ac *app.AppContext, host string) []string {
	boards, err := ac.DC.Get()
	if err != nil {
		return nil
	}
	var out []string
	for _, board := range boards {
		if board.Name == host || strings.Contains(board.Name, host) {
			out = append(out, board.Source)
		}
	}
	sort.Strings(out)
	return out
}

// unreachable renders a datasource failure as data rather than an error.
//
// sys-mcp is also the tool you reach for when Prometheus itself is broken,
// so an observability call must still answer with what static config knows
// instead of failing outright.
//
// An unset address and a dead datasource are reported differently on purpose:
// the first is a provisioning bug in modules/cli/mcp.nix, the second is a live
// incident. One note covering both sends you to the wrong place.
func unreachable(err error) map[string]any {
	note := "datasource unreachable — this says nothing about the target's " +
		"health. Check the prometheus/loki containers."
	if errors.Is(err, cache.ErrUnsetPrometheus) ||
		errors.Is(err, cache.ErrUnsetLoki) {
		note = "address never configured — this is a sys-mcp provisioning " +
			"gap, not an outage. Check the env block in modules/cli/mcp.nix."
	}
	return map[string]any{
		"reporting": false,
		"error":     err.Error(),
		"note":      note,
	}
}

func errString(err error) string {
	if err == nil {
		return "ok"
	}
	return err.Error()
}

func contains(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}
