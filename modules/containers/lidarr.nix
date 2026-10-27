# lidarr — music collection manager for Usenet and BitTorrent users
# https://lidarr.audio
{ inputs, ... }:
{
  den.aspects.containers._.lidarr = {
    aspect-name = "containers.lidarr";
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
        managarrSidecarExe = lib.getExe pkgs.homelab.managarr-lidarr;
        hostConfig = config;
      in
      {
        options.containerConfigs.lidarr =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 8686;
            defaultUid = 5004;
            defaultGid = 5004;
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
              default = "lidarr";
              description = "PostgreSQL user for lidarr";
            };
            dbName = lib.mkOption {
              type = lib.types.str;
              default = "lidarr";
              description = "PostgreSQL main database name";
            };
            dbLogName = lib.mkOption {
              type = lib.types.str;
              default = "lidarr_logs";
              description = "PostgreSQL log database name";
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
            configuratarr = lib.mkOption {
              default = { };
              description = "Declarative configuratarr sync for this Lidarr instance";
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
              description = "exportarr Prometheus exporter sidecar for this Lidarr instance";
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
                Port for managarr-lidarr, the sidecar serving this service's
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
          systemd.services."container@lidarr" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.lidarr.memoryHigh != null
            ) config.containerConfigs.lidarr.memoryHigh;
          };

          containers.lidarr =
            let
              cfg = hostConfig.containerConfigs.lidarr;
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
                  "/run/secrets/lidarr-env" = {
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

                      systemd.services.managarr-lidarr = {
                        description = "Serve Lidarr's history and notifications to managarr";
                        after = [ "lidarr.service" ];
                        wantedBy = [ "multi-user.target" ];
                        wants = [ "lidarr.service" ];

                        script = ''
                          export HOMELAB_MANAGARR_LIDARR_API_KEY="''${LIDARR__AUTH__APIKEY:-}"
                        ''
                        + "exec "
                        + lib.concatStringsSep " " (
                          [
                            managarrSidecarExe
                            "--addr :${toString cfg.apiPort}"
                            "--metrics.addr 127.0.0.1:9909"
                          ]
                          ++ lib.optional (cfg.managarrUrl != "") "--managarr-url ${cfg.managarrUrl}"
                          ++ [ "--upstream-url http://127.0.0.1:8686" ]
                        );

                        serviceConfig = {
                          User = "lidarr";
                          Group = "lidarr";
                          EnvironmentFile = "/run/secrets/lidarr-env";
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
                  # (moduleWithSystem) and orders after lidarr.service; waitForHealthy
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

                  services.lidarr = {
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
                    lidarr = {
                      preStart = lib.mkBefore ''
                        until ${pkgs.netcat-openbsd}/bin/nc -z ${cfg.dbHost} ${toString cfg.dbPort}; do
                          echo "Waiting for PostgreSQL database..."
                          sleep 1
                        done
                      '';
                      serviceConfig = {
                        StateDirectory = "lidarr";
                        User = "lidarr";
                        Group = "lidarr";
                        EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/lidarr-env";
                      };
                    };

                    # Feed the shared api-key env into the oneshot so
                    # ${env.LIDARR__AUTH__APIKEY} resolves against the same secret Lidarr uses.
                    configuratarr = lib.mkIf cfg.configuratarr.enable {
                      serviceConfig.EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/lidarr-env";
                    };

                    # Prometheus exporter sidecar — APIKEY from the same lidarr-env secret.
                    lidarr-exporter = lib.mkIf cfg.exportarr.enable {
                      description = "Lidarr Prometheus exporter (exportarr)";
                      after = [ "lidarr.service" ];
                      wantedBy = [ "multi-user.target" ];
                      environment = {
                        URL = "http://127.0.0.1:${toString cfg.port}";
                        PORT = toString cfg.exportarr.port;
                      };
                      serviceConfig = {
                        ExecStart = "${pkgs.exportarr}/bin/exportarr lidarr";
                        EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/lidarr-env";
                        DynamicUser = true;
                        Restart = "on-failure";
                        RestartSec = 10;
                      };
                    };
                  };

                  users.users.lidarr = {
                    uid = lib.mkForce cfg.uid;
                    isSystemUser = lib.mkForce false;
                    isNormalUser = true;
                  };
                  users.groups.lidarr.gid = lib.mkForce cfg.gid;

                };
            };
        };
      };
  };
}
