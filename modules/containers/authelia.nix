/*
  authelia — open-source authentication and authorization server
  https://www.authelia.com
*/
_: {
  den.aspects.containers._.authelia = {
    aspect-name = "containers.authelia";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.authelia =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 9091;
            defaultUid = 5017;
            defaultGid = 5017;
          })
          // {
            sessionDomain = lib.mkOption {
              type = lib.types.str;
              description = "Domain for Authelia session cookies (e.g. caddy.apps.internal)";
            };
            dbHost = lib.mkOption {
              type = lib.types.str;
              description = "PostgreSQL host IP";
            };
            dbPort = lib.mkOption {
              type = lib.types.port;
              default = 5432;
              description = "PostgreSQL port";
            };
            dbUser = lib.mkOption {
              type = lib.types.str;
              default = "authelia";
              description = "PostgreSQL user for Authelia";
            };
            dbName = lib.mkOption {
              type = lib.types.str;
              default = "authelia";
              description = "PostgreSQL database name for Authelia";
            };
            autheliaUrl = lib.mkOption {
              type = lib.types.str;
              description = "Authelia portal URL (e.g. http://authelia.caddy.apps.internal)";
            };
            jwtSecretFile = lib.mkOption {
              type = lib.types.str;
              description = "Path to JWT secret file (inside container)";
            };
            storageEncryptionKeyFile = lib.mkOption {
              type = lib.types.str;
              description = "Path to storage encryption key file (inside container)";
            };
            oidcHmacSecretFile = lib.mkOption {
              type = lib.types.str;
              description = "Path to OIDC HMAC secret file (inside container)";
            };
            oidcIssuerPrivateKeyFile = lib.mkOption {
              type = lib.types.str;
              description = "Path to OIDC RSA private key file (inside container)";
            };
            ldapAddress = lib.mkOption {
              type = lib.types.str;
              description = "LDAP server address (e.g. ldap://10.10.140.15:3890)";
            };
            ldapBaseDN = lib.mkOption {
              type = lib.types.str;
              description = "LDAP base DN (e.g. dc=homelab)";
            };
            ldapUser = lib.mkOption {
              type = lib.types.str;
              description = "LDAP bind (search) user DN";
            };
            ldapPasswordFile = lib.mkOption {
              type = lib.types.str;
              description = "In-container path to the LDAP bind password file";
            };
            oidcClients = lib.mkOption {
              type = lib.types.listOf lib.types.anything;
              default = [ ];
              description = "Authelia OIDC client registrations";
            };
            accessRules = lib.mkOption {
              type = lib.types.listOf lib.types.anything;
              default = [ ];
              description = "Authelia access_control rules";
            };
            metricsPort = lib.mkOption {
              type = lib.types.nullOr lib.types.port;
              default = null;
              description = "Port to expose Prometheus metrics on (null = disabled)";
            };
          };

        config = {
          systemd.services."container@authelia" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.authelia.memoryHigh != null
            ) config.containerConfigs.authelia.memoryHigh;
          };

          containers.authelia =
            let
              cfg = hostConfig.containerConfigs.authelia;
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
                { lib, ... }:
                {
                  imports = [ (import ./_base.nix { inherit lib cfg; }) ];

                  users.users."authelia-main".uid = lib.mkForce cfg.uid;
                  users.groups."authelia-main".gid = lib.mkForce cfg.gid;

                  systemd.services.authelia-main.serviceConfig.ReadWritePaths = [ "/var/lib/authelia" ];

                  services.authelia.instances.main = {
                    enable = true;
                    settings = {
                      server = {
                        address = "tcp://0.0.0.0:${toString cfg.port}/";
                        buffers = {
                          read = 65536;
                          write = 65536;
                        };
                      };
                      log = {
                        level = "info";
                        format = "json";
                        keep_stdout = true;
                      };
                      session.cookies = [
                        {
                          domain = cfg.sessionDomain;
                          authelia_url = cfg.autheliaUrl;
                          # Without these, Authelia's defaults apply — inactivity 5m,
                          # expiration 1h — which drop you back at the login page after a
                          # few idle minutes. Sessions are still memory-only (no Redis),
                          # so a container restart/deploy logs everyone out regardless.
                          inactivity = "1d";
                          expiration = "1w";
                          remember_me = "1M";
                        }
                      ];
                      storage.postgres = {
                        host = cfg.dbHost;
                        port = cfg.dbPort;
                        database = cfg.dbName;
                        username = cfg.dbUser;
                      };
                      authentication_backend.ldap = {
                        implementation = "lldap";
                        address = cfg.ldapAddress;
                        base_dn = cfg.ldapBaseDN;
                        user = cfg.ldapUser;
                      };
                      access_control = {
                        default_policy = "deny";
                        rules = cfg.accessRules;
                      };
                      notifier.filesystem.filename = "/var/lib/authelia/notification.txt";
                      identity_providers.oidc.clients = cfg.oidcClients;
                      telemetry.metrics = lib.mkIf (cfg.metricsPort != null) {
                        enabled = true;
                        address = "tcp://0.0.0.0:${toString cfg.metricsPort}/";
                      };
                    };
                    secrets = {
                      inherit (cfg) jwtSecretFile storageEncryptionKeyFile;
                      inherit (cfg) oidcHmacSecretFile oidcIssuerPrivateKeyFile;
                    };
                    environmentVariables.AUTHELIA_AUTHENTICATION_BACKEND_LDAP_PASSWORD_FILE = cfg.ldapPasswordFile;
                  };
                };
            };
        };
      };
  };
}
