# pgadmin — Web-based administration interface for PostgreSQL
_: {
  den.aspects.containers._.pgadmin = {
    aspect-name = "containers.pgadmin";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.pgadmin =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 5050;
            defaultUid = 5002;
            defaultGid = 5002;
          })
          // {
            initialEmail = lib.mkOption {
              type = lib.types.str;
              default = "admin@homelab";
              description = "Initial admin email address";
            };
            initialPasswordFile = lib.mkOption {
              type = lib.types.str;
              default = "/run/secrets/pgadmin-password";
              description = "Path to the initial admin password file";
            };
            servers = lib.mkOption {
              type = lib.types.attrsOf lib.types.anything;
              default = { };
              description = "Server definitions to pre-configure in pgAdmin";
            };
          };

        config = {
          systemd.services."container@pgadmin" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.pgadmin.memoryHigh != null
            ) config.containerConfigs.pgadmin.memoryHigh;
          };

          containers.pgadmin =
            let
              cfg = hostConfig.containerConfigs.pgadmin;
              inherit (cfg) port;
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
              extraFlags = [
                "--load-credential=pgadmin-password:${cfg.initialPasswordFile}"
              ];

              config =
                {
                  config,
                  pkgs,
                  lib,
                  ...
                }:
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                  ];

                  services.pgadmin = {
                    enable = true;
                    inherit (cfg) initialEmail;
                    initialPasswordFile = "/run/host/credentials/pgadmin-password";
                    inherit port;
                    settings.DEFAULT_SERVER =
                      if cfg.localAddress != null then lib.head (lib.splitString "/" cfg.localAddress) else "127.0.0.1";
                  };

                  services.postgresql.package = pkgs.postgresql;

                  systemd.services.pgadmin.serviceConfig.DynamicUser = lib.mkForce false;
                  users.users.pgadmin.uid = cfg.uid;
                  users.groups.pgadmin.gid = cfg.gid;

                  environment.etc."pgadmin/servers.json" = lib.mkIf (cfg.servers != { }) {
                    text = builtins.toJSON cfg.servers;
                    mode = "0600";
                    user = "pgadmin";
                    group = "pgadmin";
                  };

                  systemd.services.pgadmin.postStart = lib.mkIf (cfg.servers != { }) ''
                    for i in {1..10}; do
                      if [ -f /var/lib/pgadmin/pgadmin4.db ]; then
                        ${config.services.pgadmin.package}/bin/pgadmin4-cli load-servers /etc/pgadmin/servers.json --user ${lib.escapeShellArg cfg.initialEmail} --replace
                        break
                      fi
                      sleep 1
                    done
                  '';

                };
            };
        };
      };
  };
}
