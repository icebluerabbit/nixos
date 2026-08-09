# sonarr — TV series collection manager for Usenet and BitTorrent users
# https://sonarr.tv
{ inputs, ... }:
{
  den.aspects.containers._.sonarr = {
    aspect-name = "containers.sonarr";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.sonarr =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 8989;
            defaultUid = 5006;
            defaultGid = 5006;
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
              default = "sonarr";
              description = "PostgreSQL user for sonarr";
            };
            dbName = lib.mkOption {
              type = lib.types.str;
              default = "sonarr";
              description = "PostgreSQL main database name";
            };
            dbLogName = lib.mkOption {
              type = lib.types.str;
              default = "sonarr_logs";
              description = "PostgreSQL log database name";
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
            configuratarr = lib.mkOption {
              default = { };
              description = "Declarative configuratarr sync for this Sonarr instance";
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
              description = "exportarr Prometheus exporter sidecar for this Sonarr instance";
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
          };

        config = {
          systemd.services."container@sonarr" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.sonarr.memoryHigh != null
            ) config.containerConfigs.sonarr.memoryHigh;
          };

          containers.sonarr =
            let
              cfg = hostConfig.containerConfigs.sonarr;
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
                  "/run/secrets/sonarr-env" = {
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
                    (import ./_base.nix { inherit lib cfg; })
                    inputs.configuratarr.nixosModules.default
                  ];

                  # Declarative *arr config sync. Module self-provides its package
                  # (moduleWithSystem) and orders after sonarr.service; waitForHealthy
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

                  services.sonarr = {
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
                    sonarr = {
                      preStart = lib.mkBefore ''
                        until ${pkgs.netcat-openbsd}/bin/nc -z ${cfg.dbHost} ${toString cfg.dbPort}; do
                          echo "Waiting for PostgreSQL database..."
                          sleep 1
                        done
                      '';
                      serviceConfig = {
                        User = "sonarr";
                        Group = "sonarr";
                        EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/sonarr-env";
                      };
                    };

                    # Feed the shared api-key env into the oneshot so
                    # ${env.SONARR__AUTH__APIKEY} resolves against the same secret Sonarr uses.
                    configuratarr = lib.mkIf cfg.configuratarr.enable {
                      serviceConfig.EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/sonarr-env";
                    };

                    # Prometheus exporter sidecar — APIKEY from the same sonarr-env secret.
                    sonarr-exporter = lib.mkIf cfg.exportarr.enable {
                      description = "Sonarr Prometheus exporter (exportarr)";
                      after = [ "sonarr.service" ];
                      wantedBy = [ "multi-user.target" ];
                      environment = {
                        URL = "http://127.0.0.1:${toString cfg.port}";
                        PORT = toString cfg.exportarr.port;
                      };
                      serviceConfig = {
                        ExecStart = "${pkgs.exportarr}/bin/exportarr sonarr";
                        EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/sonarr-env";
                        DynamicUser = true;
                        Restart = "on-failure";
                        RestartSec = 10;
                      };
                    };
                  };

                  users.users.sonarr = {
                    uid = lib.mkForce cfg.uid;
                    isSystemUser = lib.mkForce false;
                    isNormalUser = true;
                  };
                  users.groups.sonarr.gid = lib.mkForce cfg.gid;

                };
            };
        };
      };
  };
}
