package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Named so a missing address is distinguishable from a datasource that is
// configured but down. The first is a provisioning bug in mcp.nix; the second
// is a live incident. Reporting them the same way sends you to the wrong place.
var (
	ErrUnsetPrometheus = errors.New("PROMETHEUS_URL is unset")
	ErrUnsetLoki       = errors.New("LOKI_URL is unset")
)

// Datasource talks to Prometheus and Loki over their HTTP APIs.
//
// Addresses arrive as PROMETHEUS_URL and LOKI_URL, set in modules/cli/mcp.nix
// from the infra client registry — the same source the containers are built
// from, resolved once at build time rather than per call.
type Datasource struct {
	Prometheus string
	Loki       string
	HTTP       *http.Client
}

func NewDatasource(prom, loki string, timeout time.Duration) *Datasource {
	return &Datasource{
		Prometheus: strings.TrimRight(prom, "/"),
		Loki:       strings.TrimRight(loki, "/"),
		HTTP:       &http.Client{Timeout: timeout},
	}
}

// Sample is one result row: its labels and the scalar value.
type Sample struct {
	Labels map[string]string `json:"labels"`
	Value  float64           `json:"value"`
}

type vectorResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			Value  []any             `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type valuesResponse struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

func (d *Datasource) get(endpoint string, q url.Values, out any) error {
	full := endpoint
	if len(q) > 0 {
		full += "?" + q.Encode()
	}
	resp, err := d.HTTP.Get(full)
	if err != nil {
		return fmt.Errorf("unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Query runs an instant PromQL query and returns the result vector.
func (d *Datasource) Query(expr string) ([]Sample, error) {
	if d.Prometheus == "" {
		return nil, ErrUnsetPrometheus
	}
	var body vectorResponse
	err := d.get(
		d.Prometheus+"/api/v1/query",
		url.Values{"query": {expr}},
		&body,
	)
	if err != nil {
		return nil, err
	}
	if body.Status != "success" {
		return nil, fmt.Errorf("prometheus rejected %q", expr)
	}

	out := make([]Sample, 0, len(body.Data.Result))
	for _, row := range body.Data.Result {
		// An instant vector encodes the value as [unixSeconds,
		// "stringifiedFloat"] — the number arrives as a string so large
		// counters survive JSON intact.
		if len(row.Value) != 2 {
			continue
		}
		raw, ok := row.Value[1].(string)
		if !ok {
			continue
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			continue
		}
		out = append(out, Sample{Labels: row.Metric, Value: v})
	}
	return out, nil
}

// MetricNames lists every metric name Prometheus currently knows.
func (d *Datasource) MetricNames() ([]string, error) {
	if d.Prometheus == "" {
		return nil, ErrUnsetPrometheus
	}
	var body valuesResponse
	err := d.get(d.Prometheus+"/api/v1/label/__name__/values", nil, &body)
	if err != nil {
		return nil, err
	}
	sort.Strings(body.Data)
	return body.Data, nil
}

// LokiLabels lists Loki's stream label names.
func (d *Datasource) LokiLabels() ([]string, error) {
	if d.Loki == "" {
		return nil, ErrUnsetLoki
	}
	var body valuesResponse
	if err := d.get(d.Loki+"/loki/api/v1/labels", nil, &body); err != nil {
		return nil, err
	}
	sort.Strings(body.Data)
	return body.Data, nil
}

// LokiValues lists the values Loki has seen for one stream label.
func (d *Datasource) LokiValues(label string) ([]string, error) {
	if d.Loki == "" {
		return nil, ErrUnsetLoki
	}
	var body valuesResponse
	endpoint := fmt.Sprintf(
		"%s/loki/api/v1/label/%s/values", d.Loki, url.PathEscape(label),
	)
	if err := d.get(endpoint, nil, &body); err != nil {
		return nil, err
	}
	sort.Strings(body.Data)
	return body.Data, nil
}
