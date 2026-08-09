# Prometheus datasource. uid pinned to its existing (postgres) uid — dashboards
# reference it by name (resolved via fallback) but alert rules resolve by real
# uid, so the alerts/ rules use this same "PBFA97CFB590B2093".
{ config }:
{
  name = "Prometheus";
  uid = "PBFA97CFB590B2093";
  type = "prometheus";
  access = "proxy";
  url = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}";
  isDefault = false;
}
