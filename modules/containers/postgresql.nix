# postgresql — Central PostgreSQL Database Service
_: {
  den.aspects.containers._.postgresql = {
    aspect-name = "containers.postgresql";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.postgresql =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 5432;
          })
          // {
            ensureUsers = lib.mkOption {
              type = lib.types.listOf lib.types.anything;
              default = [ ];
              description = "Users to ensure exist in the central database";
            };
            ensureDatabases = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              description = "Databases to ensure exist in the central database";
            };
            dbOwnerships = lib.mkOption {
              type = lib.types.attrsOf lib.types.str;
              default = { };
              description = "Map of databases to their owners (e.g. { radarr_logs = \"radarr\"; })";
            };
            maintainRoles = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              description = ''
                Roles granted the predefined `pg_maintain` role (PG17+). For apps that run
                their own VACUUM/ANALYZE housekeeping: without it every run walks the shared
                catalogs it cannot touch and logs a WARNING per catalog.
              '';
            };
            auth = lib.mkOption {
              type = lib.types.listOf (
                lib.types.submodule {
                  options = {
                    database = lib.mkOption {
                      type = lib.types.str;
                      description = "Database name";
                    };
                    user = lib.mkOption {
                      type = lib.types.str;
                      description = "User allowed to connect";
                    };
                    ip = lib.mkOption {
                      type = lib.types.str;
                      description = "IP address of the client (will automatically append /32 if no mask is provided)";
                    };
                  };
                }
              );
              default = [ ];
              description = "Structured database client authentication rules";
            };
            exporterPort = lib.mkOption {
              type = lib.types.nullOr lib.types.port;
              default = null;
              description = "Port for prometheus-postgres-exporter. Null disables it.";
            };
            extraSettings = lib.mkOption {
              type = lib.types.attrsOf lib.types.anything;
              default = { };
              description = "Additional postgresql.conf settings merged with defaults";
            };
          };

        config = {
          systemd.services."container@postgresql" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.postgresql.memoryHigh != null
            ) config.containerConfigs.postgresql.memoryHigh;
          };

          containers.postgresql =
            let
              cfg = hostConfig.containerConfigs.postgresql;
              inherit (cfg) port;
              dbDir = "/var/lib/postgresql";
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
                  ];

                  services.postgresql = {
                    enable = true;
                    enableJIT = true;
                    # Pinned explicitly: `pkgs.postgresql` floats with nixpkgs and a
                    # major bump (17→18) silently breaks the cluster (data dir is
                    # version-locked). Bump this deliberately + run pg_upgrade.
                    package = pkgs.postgresql_17;
                    dataDir = dbDir;
                    settings = {
                      listen_addresses =
                        if cfg.localAddress != null then
                          let
                            ip = lib.head (lib.splitString "/" cfg.localAddress);
                          in
                          lib.mkForce "localhost, ${ip}"
                        else
                          lib.mkForce "localhost";
                      inherit port;
                    }
                    // cfg.extraSettings;
                    authentication =
                      let
                        authLines = map (
                          rule:
                          let
                            cidr = if lib.strings.hasInfix "/" rule.ip then rule.ip else "${rule.ip}/32";
                          in
                          "host ${rule.database} ${rule.user} ${cidr} trust"
                        ) cfg.auth;
                      in
                      ''
                        # type database user address method
                        local all all trust
                        host all all 127.0.0.1/32 trust
                        host all all ::1/128 trust
                      ''
                      + "\n"
                      + lib.concatStringsSep "\n" authLines;

                    inherit (cfg) ensureUsers;
                    inherit (cfg) ensureDatabases;
                  };

                  services.prometheus.exporters.postgres = lib.mkIf (cfg.exporterPort != null) {
                    enable = true;
                    port = cfg.exporterPort;
                    dataSourceName = "postgresql://postgres@localhost:${toString port}/postgres?sslmode=disable";
                  };

                  systemd.services.postgresql-db-ownership = {
                    description = "PostgreSQL post-start database ownership alignment";
                    after = [
                      "postgresql.service"
                      "postgresql-setup.service"
                    ];
                    requires = [ "postgresql-setup.service" ];
                    wantedBy = [ "multi-user.target" ];
                    serviceConfig = {
                      Type = "oneshot";
                      User = "postgres";
                      RemainAfterExit = true;
                    };
                    script =
                      let
                        psql = lib.getExe' pkgs.postgresql "psql";
                        commands = lib.mapAttrsToList (db: owner: ''
                          ${psql} -tAc "ALTER DATABASE \"${db}\" OWNER TO ${owner};"
                          ${psql} -d "${db}" -tAc "ALTER SCHEMA public OWNER TO ${owner};"
                        '') cfg.dbOwnerships;
                        grants = map (role: ''
                          ${psql} -tAc "GRANT pg_maintain TO ${role};"
                        '') cfg.maintainRoles;
                      in
                      lib.concatStringsSep "\n" (commands ++ grants);
                  };

                };
            };
        };
      };
  };
}
