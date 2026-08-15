# mcp — Model Context Protocol servers via home-manager programs.mcp (upstream HM)
{ den, ... }:
{
  den.aspects.cli._.mcp = {
    aspect-name = "cli.mcp";
    includes = [
      den.aspects.dev._.nixd
      den.aspects.security._.sops
      den.aspects.infra._.clients._.container-grafana
      den.aspects.infra._.clients._.container-home-assistant
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.networks._.trust
    ];

    homeManager =
      {
        config,
        lib,
        osConfig,
        pkgs,
        ...
      }:
      {
        sops.secrets = {
          github-access-token-nixos = { };
          unifi-local-api-key = { };
          home-assistant-llat = { };
          grafana-token = { };
        };

        programs.mcp = {
          enable = true;
          servers = {
            nixos.command = lib.getExe pkgs.mcp-nixos;

            grafana = {
              command = toString (
                pkgs.writeShellScript "grafana-mcp" ''
                  export GRAFANA_SERVICE_ACCOUNT_TOKEN=$(cat ${config.sops.secrets.grafana-token.path})
                  exec ${lib.getExe pkgs.mcp-grafana}
                ''
              );
              env.GRAFANA_URL = "http://${osConfig.infra.clients.container-grafana.ip}:${toString osConfig.infra.clients.container-grafana.ports.default}";
            };

            github = {
              command = toString (
                pkgs.writeShellScript "github-mcp" ''
                  export GITHUB_PERSONAL_ACCESS_TOKEN=$(cat ${config.sops.secrets.github-access-token-nixos.path})
                  exec ${lib.getExe pkgs.github-mcp-server} stdio "$@"
                ''
              );
              args = [
                "--toolsets"
                "context,repos,pull_requests,issues"
              ];
            };

            fetch.command = lib.getExe pkgs.mcp-server-fetch;
            git.command = lib.getExe pkgs.mcp-server-git;
            time.command = lib.getExe pkgs.mcp-server-time;
            memory.command = lib.getExe pkgs.mcp-server-memory;
            context7.command = lib.getExe pkgs.context7-mcp;
            sequential-thinking.command = lib.getExe pkgs.mcp-server-sequential-thinking;
            sys-mcp = {
              command = toString (
                pkgs.writeShellScript "sys-mcp-wrapper" ''
                  export UNIFI_API_KEY=$(cat ${config.sops.secrets.unifi-local-api-key.path})
                  exec ${lib.getExe pkgs.homelab.sys-mcp}
                ''
              );
              env = {
                SYS_MCP_HOSTNAME = osConfig.networking.hostName;
                SYS_MCP_FLAKE_PATH = osConfig.environment.variables.FLAKE;
                UNIFI_URL = "https://${osConfig.infra.vlans.trust.staticAddresses.gateway}";
                UNIFI_SITE = "default";
                # The observability tools query these two directly rather than
                # going through Grafana, so they need no service-account token.
                PROMETHEUS_URL = "http://${osConfig.infra.clients.container-prometheus.ip}:${toString osConfig.infra.clients.container-prometheus.ports.default}";
                LOKI_URL = "http://${osConfig.infra.clients.container-loki.ip}:${toString osConfig.infra.clients.container-loki.ports.default}";
              };
            };

            home-assistant = {
              command = toString (
                pkgs.writeShellScript "ha-mcp" ''
                  export HOMEASSISTANT_TOKEN=$(cat ${config.sops.secrets.home-assistant-llat.path})
                  exec ${lib.getExe pkgs.ha-mcp}
                ''
              );
              env.HOMEASSISTANT_URL = "http://${osConfig.infra.clients.container-home-assistant.ip}:${toString osConfig.infra.clients.container-home-assistant.ports.http}";
            };

            filesystem.command = lib.getExe pkgs.mcp-server-filesystem;
          };
        };
      };
  };
}
