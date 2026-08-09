# _observability.nix — Observability tab: the metrics/logs/traces stack.
{
  cl,
  caddy,
  at,
  promUrl,
  ...
}:
let
  hc =
    {
      name,
      icon,
      href,
      desc,
      monitor ? null,
    }:
    {
      "${name}" = {
        inherit icon href;
        description = desc;
      }
      // (if monitor == null then { } else { siteMonitor = monitor; });
    };
in
[
  {
    "Observability" = [
      (hc {
        name = "Grafana";
        icon = "grafana.svg";
        href = caddy "grafana";
        monitor = at cl.container-grafana.ip cl.container-grafana.ports.default;
        desc = "Dashboards & alerting";
      })
      {
        "Prometheus" = {
          icon = "prometheus.svg";
          href = caddy "prometheus";
          widget = {
            type = "prometheus";
            url = promUrl;
            fields = [
              "targets_up"
              "targets_down"
              "targets_total"
            ];
          };
        };
      }
      {
        "Loki" = {
          icon = "mdi-file-document-multiple-#f1fa8c";
          href = caddy "grafana";
          description = "Log aggregation";
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 30000;
            metrics = [
              {
                label = "Streams";
                query = "sum(loki_ingester_memory_streams)";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Ingest";
                query = "sum(rate(loki_distributor_bytes_received_total[5m]))";
                format = {
                  type = "bytes";
                  suffix = "/s";
                };
              }
            ];
          };
        };
      }
      (hc {
        name = "Tempo";
        icon = "mdi-radar-#bd93f9";
        href = "${at cl.container-tempo.ip cl.container-tempo.ports.default}/ready";
        monitor = "${at cl.container-tempo.ip cl.container-tempo.ports.default}/ready";
        desc = "Distributed tracing backend";
      })
      {
        "UniFi Exporter" = {
          icon = "unifi.svg";
          href = caddy "grafana";
          description = "unifi-poller metrics";
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 30000;
            metrics = [
              {
                label = "Clients";
                query = "count(unpoller_client_uptime_seconds)";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
            ];
          };
        };
      }
    ];
  }
]
