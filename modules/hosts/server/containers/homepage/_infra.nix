# _infra.nix — Infrastructure tab: dev tooling, identity, databases, DNS.
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
    "Infrastructure" = [
      {
        "Gitea" = {
          icon = "gitea.svg";
          href = caddy "gitea";
          widget = {
            type = "gitea";
            url = "http://${cl.container-gitea.ip}:${toString cl.container-gitea.ports.web}";
            key = "{{HOMEPAGE_VAR_GITEA_TOKEN}}";
            fields = [
              "repositories"
              "issues"
              "pulls"
            ];
          };
        };
      }
      (hc {
        name = "Code Server";
        icon = "vscode.svg";
        href = caddy "code-server";
        monitor = at cl.container-code-server.ip cl.container-code-server.ports.web;
        desc = "Browser VS Code — shared workspace";
      })
      (hc {
        name = "OpenChamber";
        icon = "opencode.svg";
        href = caddy "openchamber";
        monitor = at cl.container-openchamber.ip cl.container-openchamber.ports.default;
        desc = "AI coding agent web UI";
      })
      {
        "OpenCode" = {
          icon = "mdi-code-braces-#8be9fd";
          href = caddy "opencode";
          siteMonitor = at cl.container-opencode.ip cl.container-opencode.ports.default;
          description = "OpenCode server";
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 60000;
            metrics = [
              {
                label = "Sessions";
                query = "sum(opencode_session_count_total)";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Cost";
                query = "sum(opencode_cost_usage_USD_total)";
                format = {
                  type = "number";
                  prefix = "$";
                  options.maximumFractionDigits = 2;
                };
              }
              {
                label = "Tokens";
                query = "sum(opencode_token_usage_tokens_total)";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
            ];
          };
        };
      }
      {
        "Authelia" = {
          icon = "authelia.svg";
          href = caddy "authelia";
          siteMonitor = at cl.container-authelia.ip cl.container-authelia.ports.default;
          description = "Authentication portal";
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 30000;
            metrics = [
              {
                label = "Req/s";
                query = "sum(rate(authelia_request[5m]))";
                format = {
                  type = "number";
                  suffix = "/s";
                  options.maximumFractionDigits = 2;
                };
              }
              {
                label = "Authz/h";
                query = "sum(increase(authelia_authz[1h]))";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "p95";
                query = "histogram_quantile(0.95, sum(rate(authelia_request_duration_bucket[5m])) by (le)) * 1000";
                format = {
                  type = "number";
                  suffix = "ms";
                  options.maximumFractionDigits = 0;
                };
              }
            ];
          };
        };
      }
      (hc {
        name = "LLDAP";
        icon = "mdi-account-key-#f1fa8c";
        href = caddy "lldap";
        monitor = at cl.container-lldap.ip cl.container-lldap.ports.http;
        desc = "LDAP directory for SSO";
      })
      (hc {
        name = "pgAdmin";
        icon = "mdi-database-cog-#8be9fd";
        href = caddy "pgadmin";
        monitor = at cl.container-pgadmin.ip cl.container-pgadmin.ports.default;
        desc = "PostgreSQL admin UI";
      })
      {
        "PostgreSQL" = {
          icon = "postgres.svg";
          href = caddy "grafana";
          description = "Primary database";
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 30000;
            metrics = [
              {
                label = "Up";
                query = "sum(pg_up)";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Conns";
                query = "sum(pg_stat_activity_count)";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Size";
                query = "sum(pg_database_size_bytes)";
                format.type = "bytes";
              }
            ];
          };
        };
      }
      {
        "DNS" = {
          icon = "mdi-dns-#50fa7b";
          href = caddy "grafana";
          description = "dnsmasq resolver";
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 30000;
            metrics = [
              {
                label = "Leases";
                query = "dnsmasq_leases";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Hits/s";
                query = "rate(dnsmasq_hits[5m])";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 1;
                };
              }
              {
                label = "Cache";
                query = "dnsmasq_cachesize";
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
