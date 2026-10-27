# radarr — movie collection manager for Usenet and BitTorrent users
# https://radarr.video
{ inputs, ... }:
{
  den.aspects.containers._.radarr = {
    aspect-name = "containers.radarr";
    includes = [ ];

    provides.to-hosts.nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        # Resolved in the HOST's package set. pkgs inside a container
        # body is the container's own and carries no overlays, so
        # pkgs.homelab there fails with "attribute 'homelab' missing".
        # The store path is what crosses the boundary.
        managarrSidecarExe = lib.getExe pkgs.homelab.managarr-radarr;
        hostConfig = config;
      in
      {
        options.containerConfigs.radarr =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 7878;
            defaultUid = 5003;
            defaultGid = 5003;
          })
          // {
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
              default = "radarr";
              description = "PostgreSQL user for radarr";
            };
            dbName = lib.mkOption {
              type = lib.types.str;
              default = "radarr";
              description = "PostgreSQL main database name";
            };
            dbLogName = lib.mkOption {
              type = lib.types.str;
              default = "radarr_logs";
              description = "PostgreSQL log database name";
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
            configuratarr = lib.mkOption {
              default = { };
              description = "Declarative configuratarr sync for this Radarr instance";
              type = lib.types.submodule {
                options = {
                  enable = lib.mkEnableOption "configuratarr declarative sync";
                  settings = lib.mkOption {
                    type = lib.types.attrs;
                    default = { };
                    description = "configuratarr settings (verbatim); each top-level key is a named instance";
                  };
                  prune = lib.mkOption {
                    type = lib.types.bool;
                    default = false;
                    description = "Delete server-side resources absent from settings";
                  };
                  waitForHealthy = lib.mkOption {
                    type = lib.types.bool;
                    default = true;
                    description = "Poll each app's health endpoint before syncing";
                  };
                  waitTimeout = lib.mkOption {
                    type = lib.types.ints.unsigned;
                    default = 120;
                    description = "Max seconds to wait per service when waitForHealthy";
                  };
                };
              };
            };
            exportarr = lib.mkOption {
              default = { };
              description = "exportarr Prometheus exporter sidecar for this Radarr instance";
              type = lib.types.submodule {
                options = {
                  enable = lib.mkEnableOption "exportarr Prometheus exporter";
                  port = lib.mkOption {
                    type = lib.types.port;
                    default = 9707;
                    description = "Port the exporter listens on (scraped by the Vector sidecar)";
                  };
                };
              };
            };
            apiPort = lib.mkOption {
              type = lib.types.port;
              default = 9910;
              description = ''
                Port for managarr-radarr, the sidecar serving this service's
                records to managarr.

                9910 on every managarr-* sidecar. Each container has its own
                IP, so one number is unambiguous everywhere and there is no
                per-service arithmetic to get wrong.
              '';
            };
            managarrUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                managarr base URL the sidecar relays events to; empty disables
                relaying and leaves the read half working.
              '';
            };
          };

        config = {
          systemd.services."container@radarr" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.radarr.memoryHigh != null
            ) config.containerConfigs.radarr.memoryHigh;
          };

          containers.radarr =
            let
              cfg = hostConfig.containerConfigs.radarr;
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
                  "/run/secrets/radarr-env" = {
                    hostPath = cfg.envFile;
                    isReadOnly = true;
                  };
                });

              config =
                {
                  pkgs,
                  lib,
                  ...
                }:
                {
                  imports = [
                    {
                      networking.firewall.allowedTCPPorts = [ cfg.apiPort ];

                      systemd.services.managarr-radarr = {
                        description = "Serve Radarr's history and notifications to managarr";
                        after = [ "radarr.service" ];
                        wantedBy = [ "multi-user.target" ];
                        wants = [ "radarr.service" ];

                        script = ''
                          export HOMELAB_MANAGARR_RADARR_API_KEY="''${RADARR__AUTH__APIKEY:-}"
                        ''
                        + "exec "
                        + lib.concatStringsSep " " (
                          [
                            managarrSidecarExe
                            "--addr :${toString cfg.apiPort}"
                            "--metrics.addr 127.0.0.1:9909"
                          ]
                          ++ lib.optional (cfg.managarrUrl != "") "--managarr-url ${cfg.managarrUrl}"
                          ++ [ "--upstream-url http://127.0.0.1:7878" ]
                        );

                        serviceConfig = {
                          User = "radarr";
                          Group = "radarr";
                          EnvironmentFile = "/run/secrets/radarr-env";
                          Restart = "on-failure";
                          RestartSec = 5;
                          ProtectSystem = "strict";
                          ProtectHome = true;
                          PrivateDevices = true;
                          PrivateTmp = true;
                          NoNewPrivileges = true;
                          RestrictAddressFamilies = [
                            "AF_INET"
                            "AF_INET6"
                          ];
                        };
                      };
                    }
                    (import ./_base.nix { inherit lib cfg; })
                    inputs.configuratarr.nixosModules.default
                  ];

                  # Declarative *arr config sync. Module self-provides its package
                  # (moduleWithSystem) and orders after radarr.service; waitForHealthy
                  # polls the API before syncing.
                  services.configuratarr = lib.mkIf cfg.configuratarr.enable {
                    enable = true;
                    inherit (cfg.configuratarr)
                      settings
                      prune
                      waitForHealthy
                      waitTimeout
                      ;
                  };

                  services.radarr = {
                    enable = true;
                    settings = {
                      postgres = {
                        host = cfg.dbHost;
                        port = cfg.dbPort;
                        maindb = cfg.dbName;
                        logdb = cfg.dbLogName;
                        user = cfg.dbUser;
                      };
                    };
                  };

                  systemd.services = {
                    radarr = {
                      preStart = lib.mkBefore ''
                        until ${pkgs.netcat-openbsd}/bin/nc -z ${cfg.dbHost} ${toString cfg.dbPort}; do
                          echo "Waiting for PostgreSQL database..."
                          sleep 1
                        done
                      '';
                      serviceConfig = {
                        StateDirectory = "radarr";
                        User = "radarr";
                        Group = "radarr";
                        EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/radarr-env";
                      };
                    };

                    # Feed the shared api-key env into the oneshot so
                    # ${env.RADARR__AUTH__APIKEY} resolves against the same secret Radarr uses.
                    configuratarr = lib.mkIf cfg.configuratarr.enable {
                      serviceConfig.EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/radarr-env";
                    };

                    # Prometheus exporter sidecar — queries the local Radarr API and
                    # exposes metrics on cfg.exportarr.port for the Vector scrape. APIKEY
                    # comes from the same radarr-env secret Radarr uses.
                    radarr-exporter = lib.mkIf cfg.exportarr.enable {
                      description = "Radarr Prometheus exporter (exportarr)";
                      after = [ "radarr.service" ];
                      wantedBy = [ "multi-user.target" ];
                      environment = {
                        URL = "http://127.0.0.1:${toString cfg.port}";
                        PORT = toString cfg.exportarr.port;
                      };
                      serviceConfig = {
                        ExecStart = "${pkgs.exportarr}/bin/exportarr radarr";
                        EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/radarr-env";
                        DynamicUser = true;
                        Restart = "on-failure";
                        RestartSec = 10;
                      };
                    };
                  };

                  users.users.radarr = {
                    uid = lib.mkForce cfg.uid;
                    isSystemUser = lib.mkForce false;
                    isNormalUser = true;
                  };
                  users.groups.radarr.gid = lib.mkForce cfg.gid;

                };
            };
        };
      };
  };
}
