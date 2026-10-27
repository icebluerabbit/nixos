# modules/containers/grafana.nix
_: {
  den.aspects.containers._.grafana = {
    aspect-name = "containers.grafana";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      {
        options.containerConfigs.grafana =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 3000;
            defaultUid = 5200;
            defaultGid = 5200;
          })
          // {
            lokiAddress = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "Loki data source endpoint URL";
            };
            prometheusAddress = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "Prometheus data source endpoint URL";
            };
            tempoAddress = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "Tempo data source endpoint URL";
            };
            enableTelemetry = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Enable OpenTelemetry tracing for Grafana internal operations";
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
            dbHost = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "Central PostgreSQL database host IP";
            };
            dbPort = lib.mkOption {
              type = lib.types.port;
              default = 5432;
              description = "Central PostgreSQL database host port";
            };
            dbUser = lib.mkOption {
              type = lib.types.str;
              default = "grafana";
              description = "PostgreSQL user for Grafana";
            };
            dbName = lib.mkOption {
              type = lib.types.str;
              default = "grafana";
              description = "PostgreSQL database name";
            };
            settings = lib.mkOption {
              type = lib.types.attrs;
              default = { };
              description = "Additional Grafana configuration settings";
            };
            provision = lib.mkOption {
              type = lib.types.attrs;
              default = { };
              description = "Additional Grafana provisioning settings (dashboards, alerts, etc.)";
            };
          };

        config = {
          systemd.services."container@grafana" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.grafana.memoryHigh != null
            ) config.containerConfigs.grafana.memoryHigh;
          };

          containers.grafana =
            let
              cfg = config.containerConfigs.grafana;
            in
            {
              inherit (cfg) autoStart;
              ephemeral = true;
              timeoutStartSec = "5min";
              privateNetwork = true;
              inherit (cfg) hostBridge;
              localMacAddress = cfg.macAddress;
              inherit (cfg) localAddress;
              bindMounts =
                cfg.bindMounts
                // (lib.optionalAttrs (cfg.envFile != null) {
                  "/run/secrets/grafana-env" = {
                    hostPath = cfg.envFile;
                    isReadOnly = true;
                  };
                });

              config =
                { pkgs, lib, ... }:
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                  ];

                  users.users.grafana = {
                    uid = lib.mkForce cfg.uid;
                    group = "grafana";
                    isSystemUser = true;
                  };
                  users.groups.grafana.gid = lib.mkForce cfg.gid;

                  # Server-side PNG rendering of dashboards, consumed by the
                  # e-paper display over HTTP. provisionGrafana is deliberately
                  # left off: it derives callback_url from http_addr, which is
                  # 0.0.0.0 here, so the renderer would call back to 0.0.0.0.
                  services.grafana-image-renderer = {
                    enable = true;
                    provisionGrafana = false;
                    # Must match rendering.renderer_token below; Grafana refuses
                    # to start if the token is left at its default "-".
                    settings.server.auth-token = "homelab-grafana-renderer-token";
                  };

                  services.grafana = {
                    enable = true;
                    declarativePlugins = with pkgs.grafanaPlugins; [
                      grafana-lokiexplore-app
                      grafana-exploretraces-app
                      grafana-metricsdrilldown-app
                      grafana-pyroscope-app
                    ];
                    settings = lib.recursiveUpdate {
                      server = {
                        http_addr = "0.0.0.0";
                        http_port = cfg.port;
                      };
                      database = lib.mkIf (cfg.dbHost != null) {
                        type = "postgres";
                        host = "${cfg.dbHost}:${toString cfg.dbPort}";
                        name = cfg.dbName;
                        user = cfg.dbUser;
                      };
                      security = {
                        secret_key = "a-long-random-secret-key-for-grafana-homelab";
                        disable_initial_admin_password_change = true;
                      };
                      auth = {
                        disable_login_form = true;
                      };
                      "auth.anonymous" = {
                        enabled = true;
                        org_role = "Admin";
                      };
                      "tracing.opentelemetry.otlp" = lib.mkIf cfg.enableTelemetry {
                        address = "127.0.0.1:4317";
                        propagation = "w3c";
                      };
                      rendering = {
                        server_url = "http://localhost:8081/render";
                        callback_url = "http://127.0.0.1:${toString cfg.port}";
                        renderer_token = "homelab-grafana-renderer-token";
                      };
                    } cfg.settings;
                    # Datasources, dashboards, and alerting are all provided per-host via
                    # cfg.provision — see the grafana host module's datasources/, dashboards/,
                    # and alerts/ dirs. The base stays generic (no datasource/uid knowledge).
                    provision = lib.recursiveUpdate {
                      enable = true;
                    } cfg.provision;
                  };

                  systemd.services.grafana.preStart = lib.mkIf (cfg.dbHost != null) (
                    lib.mkBefore ''
                      until ${pkgs.netcat-openbsd}/bin/nc -z ${cfg.dbHost} ${toString cfg.dbPort}; do
                        echo "Waiting for PostgreSQL database..."
                        sleep 1
                      done
                    ''
                  );

                  systemd.services.grafana.serviceConfig = {
                    EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/grafana-env";
                  };

                };
            };
        };
      };
  };
}
