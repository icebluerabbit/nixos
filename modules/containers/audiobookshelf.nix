/*
  audiobookshelf — self-hosted audiobook and podcast server
  https://www.audiobookshelf.org
*/
{ inputs, ... }:
{
  den.aspects.containers._.audiobookshelf = {
    # Node reads extra CAs only from NODE_EXTRA_CA_CERTS, so the OIDC
    # back-channel to Authelia (token/userinfo/JWKS over the internal-CA vhost)
    # fails without it. Authelia validates the Host header and answers 400 on a
    # bare IP, so talking to it over plain HTTP is not an option.
    aspect-name = "containers.audiobookshelf";
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
        options.containerConfigs.audiobookshelf =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 13378;
            defaultUid = 5040;
            defaultGid = 5040;
          })
          // {
            dataDir = lib.mkOption {
              type = lib.types.str;
              default = "audiobookshelf";
              description = "Config and metadata directory, relative to /var/lib";
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
            configuratarr = lib.mkOption {
              default = { };
              description = "Declarative configuratarr sync for this Audiobookshelf instance";
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
          systemd.services."container@audiobookshelf" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.audiobookshelf.memoryHigh != null
            ) config.containerConfigs.audiobookshelf.memoryHigh;
          };

          containers.audiobookshelf =
            let
              cfg = hostConfig.containerConfigs.audiobookshelf;
              # Resolved from host pkgs (the homelab overlay is not in the
              # container's nixpkgs). Only the URL and the key path are argv —
              # the admin credentials still arrive via EnvironmentFile.
              setupCmd = lib.escapeShellArgs [
                (lib.getExe pkgs.homelab.audiobookshelf-setup)
                "--base"
                "http://127.0.0.1:${toString cfg.port}"
                "--key-file"
                "/run/audiobookshelf/api-key.env"
              ];
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
                  "/run/secrets/audiobookshelf-env" = {
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

                  services.audiobookshelf = {
                    enable = true;
                    user = "audiobookshelf";
                    group = "audiobookshelf";
                    openFirewall = false;
                    # Container is ipv4-only on its own bridge address; the module
                    # default binds loopback, which is unreachable from the VLAN.
                    host = "0.0.0.0";
                    inherit (cfg) port dataDir;
                  };

                  systemd.services.audiobookshelf.environment = {
                    NODE_EXTRA_CA_CERTS = "/run/ca/ca.crt";
                    # nixpkgs builds the Nuxt frontend with the base path
                    # /audiobookshelf baked in (the served HTML references
                    # /audiobookshelf/_nuxt/…), but its ExecStart passes no
                    # ROUTER_BASE_PATH, so the server mounts its routes at the
                    # root. The SPA then calls /audiobookshelf/auth/openid, which
                    # falls through to the SPA catch-all instead of starting the
                    # OIDC flow. Setting this makes both halves agree.
                    ROUTER_BASE_PATH = "/audiobookshelf";
                  };

                  services.configuratarr = lib.mkIf cfg.configuratarr.enable {
                    enable = true;
                    inherit (cfg.configuratarr)
                      settings
                      prune
                      waitForHealthy
                      waitTimeout
                      ;
                  };

                  # ⚠ SEEDING SAFETY — never run Tools → Embed Metadata on a
                  # library item. It is the only operation in this stack that
                  # writes tags into the audio files themselves, and library files
                  # are hardlinks to still-seeding torrent data in
                  # /mnt/media/complete — an in-place write corrupts the torrent.
                  # Quick-Embed skips the backup and bulk-edit applies it en masse.
                  # It is a UI action with no API surface configuratarr can pin.
                  # Scanning and metadata.json/cover.jpg writes are safe: they only
                  # ever create new files alongside the item.

                  systemd.services = {
                    # Audiobookshelf has no env var that seeds the root user, so
                    # create it over its API and mint the key configuratarr
                    # authenticates with. Idempotent across reboots: the account
                    # lives in dataDir, not the ephemeral rootfs.
                    audiobookshelf-setup = lib.mkIf cfg.configuratarr.enable {
                      description = "Initialize Audiobookshelf and export its API key";
                      after = [ "audiobookshelf.service" ];
                      requires = [ "audiobookshelf.service" ];
                      before = [ "configuratarr.service" ];
                      wantedBy = [ "multi-user.target" ];
                      # No `path` — audiobookshelf-setup pins its own curl and jq.
                      serviceConfig = {
                        Type = "oneshot";
                        RemainAfterExit = true;
                        RuntimeDirectory = "audiobookshelf";
                        RuntimeDirectoryMode = "0700";
                        # configuratarr.service reads the key file, so it has to
                        # outlive this oneshot.
                        RuntimeDirectoryPreserve = "yes";
                        EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/audiobookshelf-env";
                        ExecStart = setupCmd;
                      };
                    };

                    configuratarr = lib.mkIf cfg.configuratarr.enable {
                      after = [ "audiobookshelf-setup.service" ];
                      requires = [ "audiobookshelf-setup.service" ];
                      serviceConfig.EnvironmentFile = [
                        "/run/audiobookshelf/api-key.env"
                      ]
                      ++ lib.optional (cfg.envFile != null) "/run/secrets/audiobookshelf-env";
                    };
                  };

                  # Pin the service user to our static uid/gid (NFS + tmpfiles parity).
                  users.users.audiobookshelf.uid = lib.mkForce cfg.uid;
                  users.groups.audiobookshelf.gid = lib.mkForce cfg.gid;
                };
            };
        };
      };
  };
}
