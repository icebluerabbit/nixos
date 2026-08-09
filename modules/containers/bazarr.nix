/*
  bazarr — subtitle downloader companion for Sonarr and Radarr
  https://www.bazarr.media
*/
{ inputs, ... }:
{
  den.aspects.containers._.bazarr = {
    aspect-name = "containers.bazarr";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.bazarr =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 6767;
            defaultUid = 5028;
            defaultGid = 5028;
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
              default = "bazarr";
              description = "PostgreSQL user for bazarr";
            };
            dbName = lib.mkOption {
              type = lib.types.str;
              default = "bazarr";
              description = "PostgreSQL database name";
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
            configuratarr = lib.mkOption {
              default = { };
              description = "Declarative configuratarr sync for this Bazarr instance";
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
          };

        config = {
          systemd.services."container@bazarr" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.bazarr.memoryHigh != null
            ) config.containerConfigs.bazarr.memoryHigh;
          };

          containers.bazarr =
            let
              cfg = hostConfig.containerConfigs.bazarr;
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
                  "/run/secrets/bazarr-env" = {
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
                  # (moduleWithSystem) and orders after bazarr.service; waitForHealthy
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

                  services.bazarr = {
                    enable = true;
                    listenPort = cfg.port;
                    user = "bazarr";
                    group = "bazarr";
                    openFirewall = false;
                  };

                  systemd.services = {
                    bazarr = {
                      preStart = lib.mkBefore ''
                        until ${pkgs.netcat-openbsd}/bin/nc -z ${cfg.dbHost} ${toString cfg.dbPort}; do
                          echo "Waiting for PostgreSQL database..."
                          sleep 1
                        done
                      '';
                      # Bazarr reads Postgres wiring straight from these env vars
                      # (os.getenv in database.py) — non-secret. pg_hba is trust-by-IP,
                      # so POSTGRES_PASSWORD is a placeholder Postgres ignores.
                      environment = {
                        POSTGRES_ENABLED = "true";
                        POSTGRES_HOST = cfg.dbHost;
                        POSTGRES_PORT = toString cfg.dbPort;
                        POSTGRES_DATABASE = cfg.dbName;
                        POSTGRES_USERNAME = cfg.dbUser;
                        POSTGRES_PASSWORD = cfg.dbUser;
                      };
                      # Secret env. DYNACONF_AUTH__APIKEY seeds bazarr's own API key
                      # (dynaconf default prefix) so it's deterministic instead of
                      # random-on-first-boot; configuratarr auths with the same value.
                      serviceConfig.EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/bazarr-env";
                    };

                    # Same env → ${env.BAZARR_API_KEY}/SONARR_APIKEY/RADARR_APIKEY
                    # resolve against the secret bazarr uses.
                    configuratarr = lib.mkIf cfg.configuratarr.enable {
                      serviceConfig.EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/bazarr-env";
                    };
                  };

                  users.users.bazarr = {
                    uid = lib.mkForce cfg.uid;
                    isSystemUser = lib.mkForce false;
                    isNormalUser = true;
                  };
                  users.groups.bazarr.gid = lib.mkForce cfg.gid;
                };
            };
        };
      };
  };
}
