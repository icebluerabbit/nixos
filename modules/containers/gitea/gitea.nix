/*
  gitea — painless self-hosted Git service
  https://gitea.io
*/
_: {
  den.aspects.containers._.gitea = {
    aspect-name = "containers.gitea";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.gitea =
          (import ../_options.nix {
            inherit lib;
            defaultUid = 5011;
            defaultGid = 5011;
          })
          // {
            webPort = lib.mkOption {
              type = lib.types.port;
              default = 3000;
              description = "Gitea Web HTTP port";
            };
            sshPort = lib.mkOption {
              type = lib.types.port;
              default = 22;
              description = "Gitea SSH port";
            };
            dbHost = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "PostgreSQL database host IP";
            };
            dbPort = lib.mkOption {
              type = lib.types.port;
              default = 5432;
              description = "PostgreSQL database host port";
            };
            dbUser = lib.mkOption {
              type = lib.types.str;
              default = "gitea";
              description = "PostgreSQL user for Gitea";
            };
            dbName = lib.mkOption {
              type = lib.types.str;
              default = "gitea";
              description = "PostgreSQL database name for Gitea";
            };
            domain = lib.mkOption {
              type = lib.types.str;
              description = "Gitea server domain (e.g. gitea.caddy.apps.internal)";
            };
            rootUrl = lib.mkOption {
              type = lib.types.str;
              description = "Gitea root URL (e.g. http://gitea.caddy.apps.internal/)";
            };
            settings = lib.mkOption {
              type = lib.types.attrsOf lib.types.anything;
              default = { };
              description = "Additional Gitea configuration settings merged into services.gitea.settings";
            };
            sshAuthorizedKeys = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              example = [ "ssh-ed25519 AAAA... code-server@gitea" ];
              description = ''
                SSH public keys to register on the `admin` account at boot, for git-over-ssh
                push. Host passes the key contents (e.g. readFile of committed pubkeys). Needs
                an existing `admin` user and `settings.server.START_SSH_SERVER = true`.
              '';
            };
          };

        config = {
          systemd.services."container@gitea" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.gitea.memoryHigh != null
            ) config.containerConfigs.gitea.memoryHigh;
          };

          containers.gitea =
            let
              cfg = hostConfig.containerConfigs.gitea;
              inherit (cfg) webPort;
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
                  config,
                  ...
                }:
                {
                  imports = [
                    (import ../_base.nix {
                      inherit lib cfg;
                      hasPort = false;
                    })
                  ];

                  networking.firewall.allowedTCPPorts = [
                    webPort
                    cfg.sshPort
                  ];

                  services.gitea = {
                    enable = true;
                    appName = "Gitea: Git with a cup of tea";
                    database = {
                      type = "postgres";
                      host = cfg.dbHost;
                      port = cfg.dbPort;
                      name = cfg.dbName;
                      user = cfg.dbUser;
                      # Use the external persistent container-postgres. Without this the
                      # gitea module defaults createDatabase=true, which spins up an
                      # embedded postgres in this ephemeral container + forces the
                      # /run/postgresql socket (overriding host) -> data on the ephemeral
                      # root, wiped on every restart. container-postgres already provisions
                      # the gitea db/user/ownership + trust-by-IP auth.
                      createDatabase = false;
                    };
                    settings = lib.recursiveUpdate {
                      server = {
                        HTTP_PORT = webPort;
                        START_SSH = true;
                        SSH_PORT = cfg.sshPort;
                        DOMAIN = cfg.domain;
                        ROOT_URL = cfg.rootUrl;
                      };
                    } cfg.settings;
                  };

                  systemd.services.gitea.preStart = lib.mkBefore ''
                    until ${pkgs.netcat-openbsd}/bin/nc -z ${cfg.dbHost} ${toString cfg.dbPort}; do
                      echo "Waiting for PostgreSQL database..."
                      sleep 1
                    done
                  '';

                  users.users.gitea = {
                    uid = lib.mkForce cfg.uid;
                    isSystemUser = lib.mkForce false;
                    isNormalUser = true;
                  };
                  users.groups.gitea.gid = lib.mkForce cfg.gid;

                  # Register cfg.sshAuthorizedKeys on the admin account once the API is up
                  # (script sidecar; keys passed as args). Self-contained at the gitea layer.
                  systemd.services.gitea-register-ssh-keys = lib.mkIf (cfg.sshAuthorizedKeys != [ ]) (
                    let
                      keyHash = builtins.substring 0 12 (
                        builtins.hashString "sha256" (lib.concatStringsSep "\n" cfg.sshAuthorizedKeys)
                      );
                    in
                    {
                      description = "Register SSH pubkeys on the gitea admin account";
                      after = [ "gitea.service" ];
                      requires = [ "gitea.service" ];
                      # also pulled in by gitea.service so it (re)runs on every gitea start,
                      # not just at boot — a config reload restarts gitea but not the target.
                      wantedBy = [
                        "multi-user.target"
                        "gitea.service"
                      ];
                      path = [
                        pkgs.curl
                        pkgs.jq
                        pkgs.gawk
                        pkgs.coreutils
                        config.services.gitea.package
                      ];
                      serviceConfig = {
                        Type = "oneshot";
                        User = "gitea";
                        RemainAfterExit = true;
                        Environment = [
                          "GITEA_API=http://localhost:${toString webPort}"
                          "MARKER=/var/lib/gitea/.ssh-keys-${keyHash}"
                        ];
                        ExecStart = "${pkgs.bash}/bin/bash ${./register-ssh-keys.sh} ${lib.escapeShellArgs cfg.sshAuthorizedKeys}";
                      };
                    }
                  );
                };
            };
        };
      };
  };
}
