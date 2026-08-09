# _overview.nix — the "start of day" tab: real host telemetry (from Prometheus,
# since the in-container resources widget only sees the ephemeral namespace) plus
# the gateways and alert surfaces worth a first glance.
{
  cl,
  caddy,
  promUrl,
  dom,
  ...
}:
let
  host = ''host="server"'';
in
[
  {
    "System Vitals" = [
      {
        "Server" = {
          icon = "mdi-server-#8be9fd";
          href = caddy "grafana";
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 10000;
            metrics = [
              {
                label = "CPU";
                query = "100 - (avg(rate(node_cpu_seconds_total{${host},mode=\"idle\"}[5m])) * 100)";
                format = {
                  type = "number";
                  suffix = " %";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "RAM";
                query = "(1 - node_memory_MemAvailable_bytes{${host}} / node_memory_MemTotal_bytes{${host}}) * 100";
                format = {
                  type = "number";
                  suffix = " %";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Temp";
                query = "max(node_hwmon_temp_celsius{${host}})";
                format = {
                  type = "number";
                  suffix = " °C";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Load";
                query = "node_load1{${host}}";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 2;
                };
              }
              {
                label = "Uptime";
                query = "node_time_seconds{${host}} - node_boot_time_seconds{${host}}";
                format.type = "duration";
              }
            ];
          };
        };
      }
      {
        "Storage" = {
          icon = "mdi-harddisk-#50fa7b";
          href = caddy "grafana";
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 30000;
            metrics = [
              {
                label = "NVMe /";
                query = "(1 - node_filesystem_avail_bytes{${host},mountpoint=\"/\"} / node_filesystem_size_bytes{${host},mountpoint=\"/\"}) * 100";
                format = {
                  type = "number";
                  suffix = " %";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Media used";
                query = "node_filesystem_size_bytes{${host},mountpoint=\"/mnt/media\"} - node_filesystem_avail_bytes{${host},mountpoint=\"/mnt/media\"}";
                format.type = "bytes";
              }
              {
                label = "Data used";
                query = "node_filesystem_size_bytes{${host},mountpoint=\"/mnt/data\"} - node_filesystem_avail_bytes{${host},mountpoint=\"/mnt/data\"}";
                format.type = "bytes";
              }
            ];
          };
        };
      }
    ];
  }
  {
    "Gateways & Alerts" = [
      {
        "VPN Gateway" = {
          icon = "mdi-shield-lock-#ff79c6";
          href = caddy "grafana";
          description = "ProtonVPN WireGuard broker";
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 15000;
            metrics = [
              {
                label = "Tunnels up";
                query = "sum(vpngw_tunnel_healthy)";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Fwd port";
                query = "max(vpngw_tunnel_forwarded_port)";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Claim age";
                query = "max(vpngw_claim_age_seconds)";
                format.type = "duration";
              }
            ];
          };
        };
      }
      {
        "Grafana" = {
          icon = "grafana.svg";
          href = caddy "grafana";
          widget = {
            type = "grafana";
            version = 2;
            url = "http://${cl.container-grafana.ip}:${toString cl.container-grafana.ports.default}";
            username = "{{HOMEPAGE_VAR_GRAFANA_USERNAME}}";
            password = "{{HOMEPAGE_VAR_GRAFANA_PASSWORD}}";
            fields = [
              "dashboards"
              "totalalerts"
              "alertstriggered"
            ];
          };
        };
      }
      {
        "Pi-hole" = {
          icon = "pi-hole.svg";
          href = caddy "pi-hole" + "admin/";
          widget = {
            type = "pihole";
            url = "http://${cl.container-pi-hole.ip}";
            version = 6;
          };
        };
      }
      {
        "Caddy" = {
          icon = "caddy.svg";
          href = "http://caddy.${dom}/";
          widget = {
            type = "caddy";
            url = "http://${cl.container-caddy.ip}:${toString cl.container-caddy.ports.admin}";
            fields = [
              "upstreams"
              "requests"
              "requests_failed"
            ];
          };
        };
      }
    ];
  }
]
