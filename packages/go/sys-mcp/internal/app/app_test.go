package app

import (
	"testing"

	"homelab/sys-mcp/internal/config"
)

// The datasource addresses come straight from configuration, unset included:
// a missing address must stay empty so the tool that needs it can say so,
// rather than being defaulted to somewhere that will never answer.
func TestDatasourceUsesConfiguredAddresses(t *testing.T) {
	ac := &AppContext{Cfg: &config.Config{
		PrometheusURL: "http://prom:9090",
		LokiURL:       "http://loki:3100",
	}}

	ds := ac.Datasource()

	if ds.Prometheus != "http://prom:9090" {
		t.Errorf("Prometheus = %q", ds.Prometheus)
	}
	if ds.Loki != "http://loki:3100" {
		t.Errorf("Loki = %q", ds.Loki)
	}
	if ds.HTTP == nil || ds.HTTP.Timeout == 0 {
		t.Error("the datasource client has no timeout")
	}
}

func TestDatasourceLeavesUnsetAddressesEmpty(t *testing.T) {
	ac := &AppContext{Cfg: &config.Config{}}

	ds := ac.Datasource()

	if ds.Prometheus != "" || ds.Loki != "" {
		t.Errorf("unset addresses were defaulted: %q %q", ds.Prometheus, ds.Loki)
	}
}
