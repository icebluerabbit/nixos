# readarr — ebook and audiobook collection manager for Usenet and BitTorrent users
# https://readarr.com
_: {
  den.aspects.containers._.readarr = {
    aspect-name = "containers.readarr";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.readarr =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 8787;
            defaultUid = 5005;
            defaultGid = 5005;
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
              default = "readarr";
              description = "PostgreSQL user for readarr";
            };
            dbName = lib.mkOption {
              type = lib.types.str;
              default = "readarr";
              description = "PostgreSQL main database name";
            };
            dbLogName = lib.mkOption {
              type = lib.types.str;
              default = "readarr_logs";
              description = "PostgreSQL log database name";
            };
            dbCacheName = lib.mkOption {
              type = lib.types.str;
              default = "readarr-cache";
              description = "PostgreSQL cache database name";
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
          };

        config = {
          systemd.services."container@readarr" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.readarr.memoryHigh != null
            ) config.containerConfigs.readarr.memoryHigh;
          };

          containers.readarr =
            let
              cfg = hostConfig.containerConfigs.readarr;
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
                  "/run/secrets/readarr-env" = {
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
                  ];

                  services.readarr = {
                    enable = true;
                    settings = {
                      postgres = {
                        host = cfg.dbHost;
                        port = cfg.dbPort;
                        maindb = cfg.dbName;
                        logdb = cfg.dbLogName;
                        cachedb = cfg.dbCacheName;
                        user = cfg.dbUser;
                      };
                    };
                  };

                  systemd.services.readarr.preStart = lib.mkBefore ''
                    until ${pkgs.netcat-openbsd}/bin/nc -z ${cfg.dbHost} ${toString cfg.dbPort}; do
                      echo "Waiting for PostgreSQL database..."
                      sleep 1
                    done
                  '';

                  systemd.services.readarr.serviceConfig = {
                    StateDirectory = "readarr";
                    User = "readarr";
                    Group = "readarr";
                    EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/readarr-env";
                  };

                  users.users.readarr = {
                    uid = lib.mkForce cfg.uid;
                    isSystemUser = lib.mkForce false;
                    isNormalUser = true;
                  };
                  users.groups.readarr.gid = lib.mkForce cfg.gid;

                };
            };
        };
      };
  };
}
