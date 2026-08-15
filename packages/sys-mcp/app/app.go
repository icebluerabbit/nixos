package app

import (
	"time"

	"sys-mcp/cache"
	"sys-mcp/config"
)

type AppContext struct {
	Cfg *config.Config
	IC  *cache.InfraCache
	HC  *cache.HostsCache
	SI  *cache.SourceIndex
	// AC and DC index what the estate has already agreed about its own
	// metrics: AC the alert thresholds, DC which dashboards chart what.
	AC *cache.AlertsCache
	DC *cache.DashboardCache
}

// Datasource returns a client for Prometheus and Loki.
//
// Addresses come from PROMETHEUS_URL and LOKI_URL, set in modules/cli/mcp.nix
// from the infra client registry — the same build-time pattern GRAFANA_URL and
// HOMEASSISTANT_URL follow. Resolving them from the registry at runtime was
// the earlier design and bought nothing: a container's address cannot change
// without a rebuild, since the container is configured from that same registry.
//
// Unset is not defaulted. A missing address surfaces as a named error from the
// tool that needed it rather than a silent connection failure to nowhere.
func (ac *AppContext) Datasource() *cache.Datasource {
	return cache.NewDatasource(
		ac.Cfg.PrometheusURL,
		ac.Cfg.LokiURL,
		15*time.Second,
	)
}
