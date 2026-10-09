package cache

import (
	"regexp"
	"sort"
	"strings"
)

// metricToken matches a Prometheus-style identifier: lowercase words joined
// by underscores, at least two segments.
var metricToken = regexp.MustCompile(`\b[a-z][a-z0-9]*(?:_[a-z0-9]+)+\b`)

// promqlBuiltins are the multi-word PromQL functions and aggregation
// modifiers the token regex would otherwise mistake for metric names —
// `min_over_time(foo[1h])` contains two matches and only one is a metric.
var promqlBuiltins = map[string]bool{
	"absent_over_time":   true,
	"avg_over_time":      true,
	"changes_over_time":  true,
	"clamp_max":          true,
	"clamp_min":          true,
	"count_over_time":    true,
	"day_of_month":       true,
	"day_of_week":        true,
	"day_of_year":        true,
	"days_in_month":      true,
	"delta_over_time":    true,
	"deriv_over_time":    true,
	"histogram_count":    true,
	"histogram_fraction": true,
	"histogram_quantile": true,
	"histogram_sum":      true,
	"holt_winters":       true,
	"label_join":         true,
	"label_replace":      true,
	"last_over_time":     true,
	"max_over_time":      true,
	"min_over_time":      true,
	"predict_linear":     true,
	"present_over_time":  true,
	"quantile_over_time": true,
	"rate_over_time":     true,
	"stddev_over_time":   true,
	"stdvar_over_time":   true,
	"sum_over_time":      true,
	"time_series":        true,
	"unless_on":          true,
}

// MetricNamesIn extracts the metric names referenced by a PromQL or LogQL
// expression, dropping function names and label-matcher values.
//
// Deliberately lossy in the safe direction: it can miss an exotic reference,
// but everything it returns is checked against what Prometheus actually
// reports before being shown, so a stray token never becomes a claim.
func MetricNamesIn(expr string) []string {
	if expr == "" {
		return nil
	}
	// Strip label matchers first. `{host="qbittorrent"}` contains tokens
	// that look exactly like metric names and are not.
	return MetricTokens(stripLabelMatchers(expr))
}

// MetricTokens extracts metric-shaped identifiers without stripping braces.
//
// For scanning source files rather than expressions. A nix aspect is nested
// attrsets, so brace-stripping would eat the entire file — the label-matcher
// pass in MetricNamesIn is only correct for a bare PromQL expression.
//
// Over-matches ordinary nix identifiers by design. Harmless, because callers
// only ever ask whether a *known* metric name is present; a stray token is
// never surfaced on its own.
func MetricTokens(text string) []string {
	if text == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, token := range metricToken.FindAllString(text, -1) {
		if promqlBuiltins[token] || seen[token] {
			continue
		}
		seen[token] = true
		out = append(out, token)
	}
	sort.Strings(out)
	return out
}

func stripLabelMatchers(expr string) string {
	var b strings.Builder
	depth := 0
	for _, r := range expr {
		switch r {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
