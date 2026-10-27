// Package app is the dependency container every tool handler receives: the
// resolved configuration plus the caches that read this flake. It exists as
// its own package so tools and the server can both import it without a cycle.
package app

import (
	"time"

	"homelab/sys-mcp/internal/cache"
	"homelab/sys-mcp/internal/config"
)

// AppContext is the wired set of dependencies shared by every tool handler.
//
// The name stutters as app.AppContext, which revive would rather see as
// app.Context. It is kept because every handler signature in internal/tools
// names it, and renaming it is churn across the whole tool surface for no
// behavioural gain during the move into this workspace.
//
//nolint:revive // renaming touches every handler; not part of the port
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
