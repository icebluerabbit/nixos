/*
  lldap — lightweight LDAP directory for homelab SSO
  https://github.com/lldap/lldap
*/
_: {
  den.aspects.containers._.lldap = {
    aspect-name = "containers.lldap";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.lldap =
          (import ./_options.nix {
            inherit lib;
            defaultUid = 5027;
            defaultGid = 5027;
          })
          // {
            ldapPort = lib.mkOption {
              type = lib.types.port;
              default = 3890;
              description = "LDAP listen port";
            };
            httpPort = lib.mkOption {
              type = lib.types.port;
              default = 17170;
              description = "Web admin HTTP port";
            };
            baseDN = lib.mkOption {
              type = lib.types.str;
              description = "LDAP base DN (e.g. dc=homelab)";
            };
            httpUrl = lib.mkOption {
              type = lib.types.str;
              description = "Public HTTP URL for password-reset links";
            };
            dbHost = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "PostgreSQL host IP";
            };
            dbPort = lib.mkOption {
              type = lib.types.port;
              default = 5432;
              description = "PostgreSQL host port";
            };
            dbUser = lib.mkOption {
              type = lib.types.str;
              default = "lldap";
              description = "PostgreSQL user";
            };
            dbName = lib.mkOption {
              type = lib.types.str;
              default = "lldap";
              description = "PostgreSQL database name";
            };
            jwtSecretPath = lib.mkOption {
              type = lib.types.str;
              description = "In-container path to the JWT secret file";
            };
            adminPassPath = lib.mkOption {
              type = lib.types.str;
              description = "In-container path to the admin password file";
            };
            seedUsers = lib.mkOption {
              type = lib.types.listOf (
                lib.types.submodule {
                  options = {
                    id = lib.mkOption {
                      type = lib.types.str;
                      description = "LDAP user id (uid)";
                    };
                    email = lib.mkOption {
                      type = lib.types.str;
                      default = "";
                      description = "User email";
                    };
                    displayName = lib.mkOption {
                      type = lib.types.str;
                      default = "";
                      description = "User display name";
                    };
                    groups = lib.mkOption {
                      type = lib.types.listOf lib.types.str;
                      default = [ ];
                      description = "Groups the user belongs to (created if absent)";
                    };
                    passwordPath = lib.mkOption {
                      type = lib.types.str;
                      description = "In-container path to a file holding the user's password";
                    };
                  };
                }
              );
              default = [ ];
              description = "LDAP users to declaratively provision on start (idempotent, never deletes)";
            };
          };

        config = {
          systemd.services."container@lldap" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.lldap.memoryHigh != null
            ) config.containerConfigs.lldap.memoryHigh;
          };

          containers.lldap =
            let
              cfg = hostConfig.containerConfigs.lldap;
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
                    (import ./_base.nix {
                      inherit lib cfg;
                      hasPort = false;
                    })
                  ];

                  networking.firewall.allowedTCPPorts = [
                    cfg.ldapPort
                    cfg.httpPort
                  ];

                  services.lldap = {
                    enable = true;
                    settings = {
                      ldap_base_dn = cfg.baseDN;
                      ldap_host = "0.0.0.0";
                      ldap_port = cfg.ldapPort;
                      http_host = "0.0.0.0";
                      http_port = cfg.httpPort;
                      http_url = cfg.httpUrl;
                      database_url = "postgres://${cfg.dbUser}@${cfg.dbHost}:${toString cfg.dbPort}/${cfg.dbName}";
                      jwt_secret_file = cfg.jwtSecretPath;
                      ldap_user_dn = "admin";
                      ldap_user_email = "admin@homelab";
                      ldap_user_pass_file = cfg.adminPassPath;
                      force_ldap_user_pass_reset = "always";
                    };
                  };

                  users.users.lldap = {
                    uid = lib.mkForce cfg.uid;
                    group = config.users.groups.lldap.name;
                    isSystemUser = true;
                  };
                  users.groups.lldap.gid = lib.mkForce cfg.gid;

                  systemd.services.lldap.serviceConfig = {
                    DynamicUser = lib.mkForce false;
                    User = config.users.users.lldap.name;
                    Group = config.users.groups.lldap.name;
                  };

                  systemd.services.lldap-seed = lib.mkIf (cfg.seedUsers != [ ]) {
                    description = "Declaratively seed lldap users (idempotent, no deletes)";
                    after = [ "lldap.service" ];
                    wants = [ "lldap.service" ];
                    wantedBy = [ "multi-user.target" ];
                    path = [ pkgs.lldap-cli ];
                    serviceConfig = {
                      Type = "oneshot";
                      RemainAfterExit = true;
                    };
                    script = ''
                      set -euo pipefail
                      export LLDAP_HTTPURL="http://localhost:${toString cfg.httpPort}"
                      for _ in $(seq 1 60); do
                        lldap-cli -D admin -w "$(cat ${cfg.adminPassPath})" login >/dev/null 2>&1 && break
                        sleep 1
                      done
                      eval "$(lldap-cli -D admin -w "$(cat ${cfg.adminPassPath})" login)"
                      ${lib.concatMapStringsSep "\n" (u: ''
                        if ! lldap-cli user list | grep -qxF ${lib.escapeShellArg u.id}; then
                          lldap-cli user add ${u.id} ${u.email} -d ${lib.escapeShellArg u.displayName} -p "$(cat ${u.passwordPath})"
                        fi
                        ${lib.concatMapStringsSep "\n" (g: ''
                          lldap-cli group add ${lib.escapeShellArg g} >/dev/null 2>&1 || true
                          lldap-cli user group add ${u.id} ${lib.escapeShellArg g} >/dev/null 2>&1 || true
                        '') u.groups}
                      '') cfg.seedUsers}
                    '';
                  };
                };
            };
        };
      };
  };
}
