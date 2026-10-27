/*
  bindery — book/audiobook automation replacing Readarr: monitors authors and
  series, searches Torznab indexers, imports completed downloads by hardlink.
  https://github.com/vavallee/bindery
*/
{ inputs, ... }:
{
  den.aspects.containers._.bindery = {
    aspect-name = "containers.bindery";
    includes = [ ];

    provides.to-hosts.nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.bindery =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 8787;
            defaultUid = 5038;
            defaultGid = 5038;
          })
          // {
            dataDir = lib.mkOption {
              type = lib.types.str;
              default = "/var/lib/bindery";
              description = "Config directory; holds bindery.db and backups (local, not the NFS library)";
            };
            downloadDir = lib.mkOption {
              type = lib.types.str;
              description = "Root the download client writes completed downloads to";
            };
            libraryDir = lib.mkOption {
              type = lib.types.str;
              description = "Destination for imported ebook files";
            };
            audiobookDir = lib.mkOption {
              type = lib.types.str;
              description = "Destination for imported audiobook files";
            };
            trustedProxies = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              description = "IPs/CIDRs whose forwarded identity headers Bindery trusts (BINDERY_TRUSTED_PROXY)";
            };
            logLevel = lib.mkOption {
              type = lib.types.enum [
                "debug"
                "info"
                "warn"
                "error"
              ];
              default = "info";
              description = "Bindery log verbosity";
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
            configuratarr = lib.mkOption {
              default = { };
              description = "Declarative configuratarr sync for this Bindery instance";
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
          systemd.services."container@bindery" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.bindery.memoryHigh != null
            ) config.containerConfigs.bindery.memoryHigh;
          };

          containers.bindery =
            let
              cfg = hostConfig.containerConfigs.bindery;
              # Resolved from host pkgs — the homelab overlay is not in the
              # container's own nixpkgs. The store path pulls into the container
              # closure via ExecStart.
              binderyExe = lib.getExe pkgs.homelab.bindery;
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
                  "/run/secrets/bindery-env" = {
                    hostPath = cfg.envFile;
                    isReadOnly = true;
                  };
                });

              config =
                { lib, ... }:
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                    inputs.configuratarr.nixosModules.default
                  ];

                  services.configuratarr = lib.mkIf cfg.configuratarr.enable {
                    enable = true;
                    inherit (cfg.configuratarr)
                      settings
                      prune
                      waitForHealthy
                      waitTimeout
                      ;
                  };

                  systemd.services = {
                    # No upstream NixOS module — upstream ships a Go binary
                    # configured entirely through BINDERY_* env vars.
                    bindery = {
                      description = "Bindery book and audiobook automation";
                      wantedBy = [ "multi-user.target" ];
                      after = [ "network.target" ];

                      environment = {
                        BINDERY_PORT = toString cfg.port;
                        BINDERY_DATA_DIR = cfg.dataDir;
                        BINDERY_DB_PATH = "${cfg.dataDir}/bindery.db";
                        BINDERY_DOWNLOAD_DIR = cfg.downloadDir;
                        BINDERY_LIBRARY_DIR = cfg.libraryDir;
                        BINDERY_AUDIOBOOK_DIR = cfg.audiobookDir;
                        BINDERY_LOG_LEVEL = cfg.logLevel;
                        # The notify chain POSTs to the Vector sidecar on this
                        # container's own RFC1918 address (loopback is rejected
                        # outright by the SSRF policy). Without this, webhook
                        # URLs resolving to RFC1918 are rejected at submit time
                        # and the configuratarr apply fails as a unit.
                        BINDERY_NOTIFICATIONS_ALLOW_PRIVATE = "1";
                        # Single sign-on: Caddy's forward_auth copies Authelia's
                        # Remote-User onto the proxied request (copyHeaders in
                        # containers/caddy.nix), and Bindery trusts that header
                        # when auth mode is `proxy`, auto-provisioning the user on
                        # first sight. Upstream's default header name is
                        # X-Forwarded-User, which Authelia does not emit.
                        BINDERY_PROXY_AUTH_HEADER = "Remote-User";
                        BINDERY_PROXY_AUTO_PROVISION = "1";
                        # Without this the identity header is rejected outright —
                        # "a forged header from an untrusted IP is logged and
                        # rejected" — so proxy mode silently falls back to the
                        # login form. Must list Caddy, the only hop that sets it.
                        BINDERY_TRUSTED_PROXY = lib.concatStringsSep "," cfg.trustedProxies;
                      };

                      serviceConfig = {
                        ExecStart = binderyExe;
                        User = "bindery";
                        Group = "bindery";
                        Restart = "on-failure";
                        RestartSec = "10s";
                        # BINDERY_API_KEY seeds the initial key on first launch;
                        # afterwards it lives in bindery.db and this is the value
                        # configuratarr authenticates with. Unlike Komga and ABS,
                        # no setup oneshot is needed.
                        EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/bindery-env";
                      };
                    };

                    configuratarr = lib.mkIf cfg.configuratarr.enable {
                      after = [ "bindery.service" ];
                      requires = [ "bindery.service" ];
                      serviceConfig.EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/bindery-env";
                    };
                  };

                  users.users.bindery = {
                    uid = lib.mkForce cfg.uid;
                    group = "bindery";
                    isSystemUser = true;
                  };
                  users.groups.bindery.gid = lib.mkForce cfg.gid;
                };
            };
        };
      };
  };
}
