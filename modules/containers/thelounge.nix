/*
  thelounge — modern, self-hosted web IRC client
  https://github.com/thelounge/thelounge
*/
_: {
  den.aspects.containers._.thelounge = {
    aspect-name = "containers.thelounge";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.thelounge =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 9000;
            defaultUid = 5026;
            defaultGid = 5026;
          })
          // {
            ldapUrl = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "LDAP server URL for auth (e.g. ldap://10.10.140.15:3890); null disables";
            };
            ldapBaseDN = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "LDAP base DN for simple-bind user lookup (e.g. ou=people,dc=homelab)";
            };
            seedScript = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Path to the Python network-seed script (run in the container)";
            };
            seedUsers = lib.mkOption {
              type = lib.types.listOf (
                lib.types.submodule {
                  options = {
                    name = lib.mkOption {
                      type = lib.types.str;
                      description = "Username (must match the LDAP uid)";
                    };
                    clientSettings = lib.mkOption {
                      type = lib.types.attrs;
                      default = { };
                      description = "Per-user clientSettings merged into users/<name>.json (declared keys win).";
                    };
                    networks = lib.mkOption {
                      default = [ ];
                      type = lib.types.listOf (
                        lib.types.submodule {
                          options = {
                            name = lib.mkOption { type = lib.types.str; };
                            host = lib.mkOption { type = lib.types.str; };
                            port = lib.mkOption {
                              type = lib.types.port;
                              default = 6697;
                            };
                            tls = lib.mkOption {
                              type = lib.types.bool;
                              default = true;
                            };
                            rejectUnauthorized = lib.mkOption {
                              type = lib.types.bool;
                              default = true;
                            };
                            nickPath = lib.mkOption {
                              type = lib.types.nullOr lib.types.str;
                              default = null;
                              description = "In-container path to a file holding the nick (sops secret); null = omit, thelounge uses its own default nick";
                            };
                            passwordPath = lib.mkOption {
                              type = lib.types.nullOr lib.types.str;
                              default = null;
                              description = "In-container path to a file holding the IRC server password (sops secret); null = none";
                            };
                            channels = lib.mkOption {
                              type = lib.types.listOf lib.types.str;
                              default = [ ];
                            };
                            commands = lib.mkOption {
                              type = lib.types.listOf lib.types.str;
                              default = [ ];
                              description = ''
                                Commands run on connect. `{key}` placeholders are
                                substituted from `commandsSecretPaths` at seed time.
                              '';
                            };
                            commandsSecretPaths = lib.mkOption {
                              type = lib.types.attrsOf lib.types.str;
                              default = { };
                              description = ''
                                Map of placeholder name → in-container path to a
                                secret file; substituted into `commands` so secrets
                                stay out of the Nix store.
                              '';
                            };
                          };
                        }
                      );
                    };
                  };
                }
              );
              default = [ ];
              description = "Per-user IRC networks to enforce (merged into users/<name>.json .networks every boot)";
            };
          };

        config = {
          systemd.services."container@thelounge" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.thelounge.memoryHigh != null
            ) config.containerConfigs.thelounge.memoryHigh;
          };

          containers.thelounge =
            let
              cfg = hostConfig.containerConfigs.thelounge;
            in
            {
              inherit (cfg) autoStart;
              ephemeral = true;
              timeoutStartSec = "5min";
              privateNetwork = true;
              inherit (cfg) hostBridge;
              localMacAddress = cfg.macAddress;
              inherit (cfg) localAddress bindMounts;

              config =
                {
                  config,
                  lib,
                  pkgs,
                  ...
                }:
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                  ];

                  services.thelounge = {
                    enable = true;
                    public = false;
                    inherit (cfg) port;
                    extraConfig = {
                      # Inline link/image previews. Fetched by the container, cached
                      # and proxied locally so the browser never touches the remote
                      # host. Cache lives in the persistent storage dir, unbounded —
                      # upstream's storagePolicy prunes it only by expiring message
                      # history, which would fight maxHistory and search.
                      prefetch = true;
                      prefetchStorage = true;
                      prefetchMaxImageSize = 4096; # KB — skip previews above this
                      prefetchTimeout = 5000; # ms before giving up on a link
                      # Persistent scrollback; sqlite is what searchEnabled searches.
                      messageStorage = [
                        "sqlite"
                        "text"
                      ];
                      maxHistory = 10000; # lines held in memory per channel/query
                    }
                    // lib.optionalAttrs (cfg.ldapUrl != null) {
                      ldap = {
                        enable = true;
                        url = cfg.ldapUrl;
                        primaryKey = "uid";
                        baseDN = cfg.ldapBaseDN;
                      };
                    };
                  };

                  users.users.thelounge = {
                    uid = lib.mkForce cfg.uid;
                    group = config.users.groups.thelounge.name;
                    isSystemUser = true;
                  };
                  users.groups.thelounge.gid = lib.mkForce cfg.gid;

                  systemd.services.thelounge-seed = lib.mkIf (cfg.seedUsers != [ ] && cfg.seedScript != null) {
                    description = "Seed thelounge per-user IRC networks (merge .networks)";
                    before = [ "thelounge.service" ];
                    wantedBy = [ "multi-user.target" ];
                    serviceConfig = {
                      Type = "oneshot";
                      RemainAfterExit = true;
                      User = config.users.users.thelounge.name;
                      Group = config.users.groups.thelounge.name;
                      Environment = "THELOUNGE_SEED_FILE=${pkgs.writeText "thelounge-seed.json" (builtins.toJSON cfg.seedUsers)}";
                      ExecStart = "${pkgs.python3}/bin/python3 ${cfg.seedScript}";
                    };
                  };
                };
            };
        };
      };
  };
}
