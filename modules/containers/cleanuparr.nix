/*
  cleanuparr — automated cleanup of stalled, blocked and unlinked *arr downloads
  https://github.com/Cleanuparr/Cleanuparr

  Package + NixOS module come from icebluerabbit/cleanuparr-flake (nixpkgs has neither).
  All service configuration lives in cleanuparr's database and is edited through
  the web UI — there is no config file. Only the connection/runtime knobs below
  are declarative.
*/
{ inputs, ... }:
{
  flake-file.inputs = {
    cleanuparr-flake = {
      url = "github:icebluerabbit/cleanuparr-flake/dev";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  den.aspects.containers._.cleanuparr = {
    aspect-name = "containers.cleanuparr";
    includes = [ ];

    provides.to-hosts.nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      {
        options.containerConfigs.cleanuparr =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 11011;
            defaultUid = 5037;
            defaultGid = 5037;
          })
          // {
            dbHost = lib.mkOption {
              type = lib.types.str;
              description = "Central PostgreSQL database host IP";
            };
            dbPort = lib.mkOption {
              type = lib.types.port;
              default = 5432;
              description = "Central PostgreSQL database host port";
            };
            dbUser = lib.mkOption {
              type = lib.types.str;
              default = "cleanuparr";
              description = "PostgreSQL user for cleanuparr";
            };
            dbName = lib.mkOption {
              type = lib.types.str;
              default = "cleanuparr";
              description = "PostgreSQL database name for cleanuparr";
            };
            mediaDir = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "Media tree the download cleaner scans for unlinked and orphaned files";
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
            configuratarr = lib.mkOption {
              default = { };
              description = "Declarative configuratarr sync for this cleanuparr instance";
              type = lib.types.submodule {
                options = {
                  enable = lib.mkEnableOption "configuratarr declarative sync";
                  adminUser = lib.mkOption {
                    type = lib.types.str;
                    default = "admin";
                    description = ''
                      Cleanuparr admin account name.

                      A plain string, not a path, because this value is not a
                      secret: Cleanuparr has no external identity and the name
                      is committed in plaintext here. Services whose username
                      *is* a secret — most of this homelab's are — take a
                      file path instead; cleanuparr-setup accepts either.
                    '';
                  };
                  adminPasswordFile = lib.mkOption {
                    type = lib.types.nullOr lib.types.path;
                    default = null;
                    description = ''
                      Host path to a file holding the admin password, normally
                      a sops secret.

                      Delivered with `--load-credential` rather than through
                      the environment file: a credential is scoped to the
                      container, is not inherited by child processes, and
                      never appears in any process environment.
                    '';
                  };
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
          systemd.services."container@cleanuparr" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.cleanuparr.memoryHigh != null
            ) config.containerConfigs.cleanuparr.memoryHigh;
          };

          containers.cleanuparr =
            let
              cfg = config.containerConfigs.cleanuparr;
              cleanuparrModule = inputs.cleanuparr-flake.nixosModules.default;
              # Resolved from host pkgs (the homelab overlay is not in the
              # container's nixpkgs). The password arrives as an nspawn
              # credential, so only its in-container path is argv — the secret
              # itself never enters the command line or the environment.
              credentialPath = "/run/host/credentials/cleanuparr-admin-password";
              setupCmd = lib.escapeShellArgs [
                (lib.getExe pkgs.homelab.cleanuparr-setup)
                "--base"
                "http://127.0.0.1:${toString cfg.port}"
                "--key-file"
                "/run/cleanuparr/api-key.env"
                "--admin-user"
                cfg.configuratarr.adminUser
                "--admin-password-file"
                credentialPath
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

              # Same shape pgadmin uses: nspawn reads the host path at start
              # and exposes it inside at /run/host/credentials/<id>, on a
              # tmpfs, so the ephemeral rootfs is irrelevant.
              extraFlags = lib.optional (
                cfg.configuratarr.adminPasswordFile != null
              ) "--load-credential=cleanuparr-admin-password:${cfg.configuratarr.adminPasswordFile}";

              bindMounts =
                cfg.bindMounts
                // (lib.optionalAttrs (cfg.envFile != null) {
                  "/run/secrets/cleanuparr-env" = {
                    hostPath = cfg.envFile;
                    isReadOnly = true;
                  };
                });

              config =
                { pkgs, lib, ... }:
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                    cleanuparrModule
                    inputs.configuratarr.nixosModules.default
                  ];

                  users.users.cleanuparr.uid = lib.mkForce cfg.uid;
                  users.groups.cleanuparr.gid = lib.mkForce cfg.gid;

                  services.cleanuparr = {
                    enable = true;
                    inherit (cfg) port;
                    bindAddress = "0.0.0.0";
                    dataDir = "/var/lib/cleanuparr";
                    # _base.nix owns the container perimeter.
                    openFirewall = false;
                    database = {
                      provider = "postgres";
                      postgres = {
                        host = cfg.dbHost;
                        port = cfg.dbPort;
                        user = cfg.dbUser;
                        database = cfg.dbName;
                      };
                    };
                    # PostgreSQL authenticates this container by IP (pg_hba `trust`),
                    # so the value is never checked — but Cleanuparr refuses to start
                    # on an empty POSTGRES_PASS.
                    environment.POSTGRES_PASS = "trust";
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

                  systemd.services = {
                    cleanuparr = {
                      preStart = lib.mkBefore ''
                        until ${pkgs.netcat-openbsd}/bin/nc -z ${cfg.dbHost} ${toString cfg.dbPort}; do
                          echo "Waiting for PostgreSQL database..."
                          sleep 1
                        done
                      '';
                      # The module's ProtectSystem = "strict" makes everything outside
                      # dataDir read-only; the download cleaner deletes unlinked and
                      # orphaned files under the media tree.
                      serviceConfig.ReadWritePaths = lib.mkIf (cfg.mediaDir != null) (
                        lib.mkAfter [
                          cfg.mediaDir
                        ]
                      );
                    };

                    # Cleanuparr mints its API key with the admin account during
                    # first-run setup and only serves it from the API — there is no
                    # config file or DB row to preseed, and every non-auth /api/*
                    # path 403s "Setup required" until setup completes. Do the setup
                    # dance, then drop the key where configuratarr reads it.
                    # Idempotent across reboots: the account lives in PostgreSQL, not
                    # the ephemeral rootfs, and /api/auth/status is unauthenticated.
                    cleanuparr-setup = lib.mkIf cfg.configuratarr.enable {
                      description = "Complete Cleanuparr first-run setup and export its API key";
                      after = [ "cleanuparr.service" ];
                      requires = [ "cleanuparr.service" ];
                      before = [ "configuratarr.service" ];
                      wantedBy = [ "multi-user.target" ];
                      # No `path` — cleanuparr-setup is a static Go binary
                      # with no runtime dependencies.
                      serviceConfig = {
                        Type = "oneshot";
                        RemainAfterExit = true;
                        RuntimeDirectory = "cleanuparr";
                        RuntimeDirectoryMode = "0700";
                        # configuratarr.service reads the key file, so it has to
                        # outlive this oneshot.
                        RuntimeDirectoryPreserve = "yes";
                        # No EnvironmentFile: the admin password comes in as an
                        # nspawn credential and everything else is argv, so this
                        # unit needs nothing from the environment.
                        ExecStart = setupCmd;
                      };
                    };

                    configuratarr = lib.mkIf cfg.configuratarr.enable {
                      after = [ "cleanuparr-setup.service" ];
                      requires = [ "cleanuparr-setup.service" ];
                      serviceConfig.EnvironmentFile = [
                        "/run/cleanuparr/api-key.env"
                      ]
                      ++ lib.optional (cfg.envFile != null) "/run/secrets/cleanuparr-env";
                    };
                  };
                };
            };
        };
      };
  };
}
