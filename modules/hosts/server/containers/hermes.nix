# hermes — host configuration override for the Hermes agent gateway container
{ den, inputs, ... }:
{
  den.aspects.hosts._.server._.containers._.hermes = {
    aspect-name = "hosts.server.containers.hermes";
    includes = [
      den.aspects.containers._.hermes
      den.aspects.infra._.clients._.container-hermes
      den.aspects.infra._.clients._.container-tuwunel
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.services
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.services
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
      den.aspects.infra._.clients._.container-grafana
      den.aspects.infra._.clients._.container-home-assistant
      den.aspects.infra._.networks._.iot
    ];

    nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        cfg = config.containerConfigs.hermes;
      in
      {
        # tuwunel lives in the core wave; the gateway logs in to it on start.
        systemd.services."container@hermes" = {
          after = [ "container-wave-core.target" ];
          before = [ "container-wave-apps.target" ];
        };

        containerConfigs.hermes = {
          lldpd.enable = true;
          hostBridge = "br-services";
          # Routed through OpenRouter (single account shared with opencode); key
          # arrives as OPENROUTER_API_KEY via the hermes-env sops template.
          provider = "openrouter";
          model = "deepseek/deepseek-v4-flash";
          macAddress = config.infra.clients.container-hermes.mac;
          localAddress = "${config.infra.clients.container-hermes.ip}/${toString config.infra.vlans.services.subnetPrefix}";
          gateway = config.infra.vlans.services.staticAddresses.gateway;
          port = config.infra.clients.container-hermes.ports.default;
          envFile = config.sops.templates."hermes-env".path;

          matrixHomeserver = "http://${config.infra.clients.container-tuwunel.ip}:${toString config.infra.clients.container-tuwunel.ports.default}";
          matrixAllowedUsers = [ "@admin:${cfg.matrixServerName}" ];
          homeRoomAlias = "hermes";
          # Dedicated agent room — every message in it is addressed to hermes,
          # so mention-gating (on by default elsewhere) buys nothing here.
          freeResponseRoomAliases = [ "hermes" ];

          settings = {
            # MCP servers reachable to the agent. Both are stdio binaries (same
            # ones the CLI's Claude Code uses in cli/mcp.nix); tokens arrive via
            # ${VAR} interpolation from the hermes .env (see hermes-env template).
            mcp_servers = {
              grafana = {
                command = lib.getExe pkgs.mcp-grafana;
                env = {
                  GRAFANA_URL = "http://${config.infra.clients.container-grafana.ip}:${toString config.infra.clients.container-grafana.ports.default}";
                  GRAFANA_SERVICE_ACCOUNT_TOKEN = "\${GRAFANA_SERVICE_ACCOUNT_TOKEN}";
                };
              };
              home-assistant = {
                command = lib.getExe pkgs.ha-mcp;
                env = {
                  HOMEASSISTANT_URL = "http://${config.infra.clients.container-home-assistant.ip}:${toString config.infra.clients.container-home-assistant.ports.http}";
                  HOMEASSISTANT_TOKEN = "\${HOMEASSISTANT_TOKEN}";
                };
              };
            };

            # Deprecated MESSAGING_CWD env var replaced by config.yaml terminal.cwd —
            # set the agent's default terminal working directory declaratively.
            terminal.cwd = "/var/lib/hermes/workspace";

            # Onboarding prompts are one-shot interactive flows the app otherwise
            # persists into config.yaml after first run. Declare them seen so the
            # ephemeral root never re-triggers them on recreate.
            onboarding.seen.profile_build_offered = true;

            # caveman skill installed straight from its flake input, read-only —
            # external_dirs are scanned in place, no copy. Points at the single
            # skill dir (contains SKILL.md), so only caveman registers, not the
            # sibling Claude-Code-oriented skills (caveman-commit/review/…).
            skills.external_dirs = [ "${inputs.caveman}/skills/caveman" ];
          };

          bindMounts = {
            "/var/lib/hermes" = {
              hostPath = "/var/lib/appdata/hermes";
              isReadOnly = false;
            };
          };

          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];

          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/hermes 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/hermes 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];
      };
  };
}
