# modules/containers/youtarr.nix
{
  inputs,
  ...
}:
{
  flake-file.inputs = {
    youtarr-flake = {
      url = "github:icebluerabbit/youtarr-flake/dev";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  den.aspects.containers._.youtarr = {
    aspect-name = "containers.youtarr";
    includes = [ ];

    provides.to-hosts.nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      {
        options.containerConfigs.youtarr =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 3011;
            defaultUid = 5008;
            defaultGid = 5008;
          })
          // {
            mysqlUid = lib.mkOption {
              type = lib.types.int;
              default = 84;
            };
            mysqlGid = lib.mkOption {
              type = lib.types.int;
              default = 84;
            };
            dbHost = lib.mkOption {
              type = lib.types.str;
              default = "127.0.0.1";
            };
            dbPort = lib.mkOption {
              type = lib.types.port;
              default = 3306;
            };
            dbUser = lib.mkOption {
              type = lib.types.str;
              default = "youtarr";
            };
            dbName = lib.mkOption {
              type = lib.types.str;
              default = "youtarr";
            };
            dbPasswordFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Path to the file containing the database password.";
            };
            youtubeOutputDir = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Directory where downloaded videos are saved.";
            };
            auth = {
              enable = lib.mkOption {
                type = lib.types.bool;
                default = true;
                description = "Enable built-in authentication.";
              };
            };
            settings = inputs.youtarr-flake.lib.settingsOption { inherit lib pkgs; };
            secretSettings = lib.mkOption {
              type = lib.types.attrsOf lib.types.str;
              default = { };
              description = "Map of config.json keys to env var names; injected at runtime to keep secrets out of the store.";
            };
          };

        config = {
          systemd.services."container@youtarr" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.youtarr.memoryHigh != null
            ) config.containerConfigs.youtarr.memoryHigh;
          };

          containers.youtarr =
            let
              cfg = config.containerConfigs.youtarr;
              youtarrModule = inputs.youtarr-flake.nixosModules.default;
            in
            {
              inherit (cfg) autoStart;
              ephemeral = true;
              timeoutStartSec = "5min";
              privateNetwork = true;
              inherit (cfg) hostBridge;
              localMacAddress = cfg.macAddress;
              inherit (cfg) localAddress;
              inherit (cfg) bindMounts;

              config =
                {
                  pkgs,
                  lib,
                  ...
                }:
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                    youtarrModule
                  ];

                  services.mysql = {
                    enable = true;
                    package = pkgs.mariadb;
                    ensureDatabases = [ cfg.dbName ];
                    ensureUsers = [
                      {
                        name = cfg.dbUser;
                        ensurePermissions = {
                          "${cfg.dbName}.*" = "ALL PRIVILEGES";
                        };
                      }
                    ];
                  };

                  users = {
                    users = {
                      mysql.uid = lib.mkForce cfg.mysqlUid;
                      youtarr = {
                        uid = lib.mkForce cfg.uid;
                        group = "youtarr";
                        isSystemUser = lib.mkForce false;
                        isNormalUser = true;
                      };
                    };
                    groups = {
                      mysql.gid = lib.mkForce cfg.mysqlGid;
                      youtarr.gid = lib.mkForce cfg.gid;
                    };
                  };

                  systemd.services.youtarr-db-setup = {
                    description = "Youtarr Database User Setup";
                    after = [ "mysql.service" ];
                    requires = [ "mysql.service" ];
                    before = [ "youtarr.service" ];
                    wantedBy = [ "multi-user.target" ];

                    serviceConfig = {
                      Type = "oneshot";
                      User = "root";
                      Group = "root";
                      ExecStart = pkgs.writeShellScript "youtarr-db-setup" ''
                        until [ -S /run/mysqld/mysqld.sock ]; do sleep 1; done

                        DB_PASSWORD=""
                        if [ -n "${toString cfg.dbPasswordFile}" ] && [ -f "${toString cfg.dbPasswordFile}" ]; then
                          . "${toString cfg.dbPasswordFile}"
                        fi

                        if [ -n "$DB_PASSWORD" ]; then
                          ${pkgs.mariadb}/bin/mariadb -u root -e "
                            CREATE USER IF NOT EXISTS '${cfg.dbUser}'@'localhost' IDENTIFIED BY '$DB_PASSWORD';
                            GRANT ALL PRIVILEGES ON ${cfg.dbName}.* TO '${cfg.dbUser}'@'localhost';

                            CREATE USER IF NOT EXISTS '${cfg.dbUser}'@'127.0.0.1' IDENTIFIED BY '$DB_PASSWORD';
                            GRANT ALL PRIVILEGES ON ${cfg.dbName}.* TO '${cfg.dbUser}'@'127.0.0.1';

                            ALTER USER '${cfg.dbUser}'@'localhost' IDENTIFIED BY '$DB_PASSWORD';
                            ALTER USER '${cfg.dbUser}'@'127.0.0.1' IDENTIFIED BY '$DB_PASSWORD';

                            ${lib.optionalString (cfg.dbHost != "localhost" && cfg.dbHost != "127.0.0.1") ''
                              CREATE USER IF NOT EXISTS '${cfg.dbUser}'@'${cfg.dbHost}' IDENTIFIED BY '$DB_PASSWORD';
                              GRANT ALL PRIVILEGES ON ${cfg.dbName}.* TO '${cfg.dbUser}'@'${cfg.dbHost}';
                              ALTER USER '${cfg.dbUser}'@'${cfg.dbHost}' IDENTIFIED BY '$DB_PASSWORD';
                            ''}

                            FLUSH PRIVILEGES;
                          "
                        fi
                      '';
                    };
                  };

                  services.youtarr = {
                    enable = true;
                    inherit (cfg) port;
                    database = {
                      host = cfg.dbHost;
                      port = cfg.dbPort;
                      name = cfg.dbName;
                      user = cfg.dbUser;
                      passwordFile = cfg.dbPasswordFile;
                    };
                    youtubeOutputDir = lib.mkIf (cfg.youtubeOutputDir != null) cfg.youtubeOutputDir;
                    inherit (cfg) settings secretSettings;
                    auth.enable = cfg.auth.enable;
                  };

                  systemd.services.youtarr = {
                    after = [ "youtarr-db-setup.service" ];
                    requires = [ "youtarr-db-setup.service" ];
                  };

                };
            };
        };
      };
  };
}
