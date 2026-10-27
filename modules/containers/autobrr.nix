/*
  autobrr — IRC announce monitor and torrent downloader automation
  https://autobrr.com
*/
{ inputs, ... }:
{
  den.aspects.containers._.autobrr = {
    aspect-name = "containers.autobrr";
    includes = [ ];

    provides.to-hosts.nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        # Resolved in the HOST's package set. pkgs inside a container
        # body is the container's own and carries no overlays, so
        # pkgs.homelab there fails with "attribute 'homelab' missing".
        # The store path is what crosses the boundary.
        managarrSidecarExe = lib.getExe pkgs.homelab.managarr-autobrr;
        hostConfig = config;
      in
      {
        options.containerConfigs.autobrr =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 7474;
            defaultUid = 5024;
            defaultGid = 5024;
          })
          // {
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
            secretFile = lib.mkOption {
              type = lib.types.path;
              description = "File containing the autobrr session secret";
            };
            logLevel = lib.mkOption {
              type = lib.types.enum [
                "ERROR"
                "WARN"
                "INFO"
                "DEBUG"
                "TRACE"
              ];
              default = "INFO";
              description = "autobrr log level";
            };
            customDefinitions = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = ''
                Directory of extra indexer definition files. autobrr loads any file whose
                name contains ".yaml" from the directory named by config.toml's
                customDefinitions, on top of the definitions built into the binary — use it
                for trackers with no upstream definition.
              '';
            };
            configuratarr = lib.mkOption {
              default = { };
              description = "Declarative configuratarr sync for this autobrr instance";
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
            apiPort = lib.mkOption {
              type = lib.types.port;
              default = 9910;
              description = ''
                Port for managarr-autobrr, the sidecar serving this service's
                records to managarr.

                9910 on every managarr-* sidecar. Each container has its own
                IP, so one number is unambiguous everywhere and there is no
                per-service arithmetic to get wrong.
              '';
            };
            managarrUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                managarr base URL the sidecar relays events to; empty disables
                relaying and leaves the read half working.
              '';
            };
          };

        config = {
          systemd.services."container@autobrr" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.autobrr.memoryHigh != null
            ) config.containerConfigs.autobrr.memoryHigh;
          };

          containers.autobrr =
            let
              cfg = hostConfig.containerConfigs.autobrr;
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
                  "/run/secrets/autobrr-env" = {
                    hostPath = cfg.envFile;
                    isReadOnly = true;
                  };
                });

              config =
                {
                  pkgs,
                  lib,
                  config,
                  ...
                }:
                {
                  imports = [
                    {
                      networking.firewall.allowedTCPPorts = [ cfg.apiPort ];

                      systemd.services.managarr-autobrr = {
                        description = "Serve autobrr's announce history to managarr";
                        after = [ "autobrr.service" ];
                        wantedBy = [ "multi-user.target" ];
                        wants = [ "autobrr.service" ];

                        script = ''
                          export HOMELAB_MANAGARR_AUTOBRR_API_KEY="''${AUTOBRR_API_TOKEN:-}"
                        ''
                        + "exec "
                        + lib.concatStringsSep " " (
                          [
                            managarrSidecarExe
                            "--addr :${toString cfg.apiPort}"
                            "--metrics.addr 127.0.0.1:9909"
                          ]
                          ++ lib.optional (cfg.managarrUrl != "") "--managarr-url ${cfg.managarrUrl}"
                          ++ [ "--upstream-url http://127.0.0.1:7474" ]
                        );

                        serviceConfig = {
                          User = "autobrr";
                          Group = "autobrr";
                          EnvironmentFile = "/run/secrets/autobrr-env";
                          Restart = "on-failure";
                          RestartSec = 5;
                          ProtectSystem = "strict";
                          ProtectHome = true;
                          PrivateDevices = true;
                          PrivateTmp = true;
                          NoNewPrivileges = true;
                          RestrictAddressFamilies = [
                            "AF_INET"
                            "AF_INET6"
                          ];
                        };
                      };
                    }
                    (import ./_base.nix { inherit lib cfg; })
                    inputs.configuratarr.nixosModules.default
                  ];

                  users.users.autobrr = {
                    uid = lib.mkForce cfg.uid;
                    group = "autobrr";
                    isSystemUser = true;
                  };
                  users.groups.autobrr.gid = lib.mkForce cfg.gid;

                  services.autobrr = {
                    enable = true;
                    openFirewall = false;
                    inherit (cfg) secretFile;
                    settings = {
                      host = "0.0.0.0";
                      inherit (cfg) port;
                      checkForUpdates = false;
                      inherit (cfg) logLevel;
                    }
                    // lib.optionalAttrs (cfg.customDefinitions != null) {
                      customDefinitions = "/var/lib/autobrr/definitions";
                    };
                  };

                  # Run as our static user (the bind-mounted StateDirectory needs
                  # stable ownership) instead of the module's DynamicUser. With
                  # DynamicUser off, %S resolves to /var/lib, so relocate the module's
                  # config.toml symlink out of /var/lib/private into the real state dir.
                  systemd.tmpfiles.settings."10-autobrr" = lib.mkForce (
                    {
                      "/var/lib/autobrr/config.toml"."L+".argument = "${(pkgs.formats.toml { }).generate "autobrr.toml"
                        config.services.autobrr.settings
                      }";
                    }
                    # Symlinked into the bind-mounted state dir for the same reason as
                    # config.toml above: customDefinitions must be a path autobrr can read
                    # at startup, and /nix/store is bind-mounted read-only into the container.
                    // lib.optionalAttrs (cfg.customDefinitions != null) {
                      "/var/lib/autobrr/definitions"."L+".argument = "${cfg.customDefinitions}";
                    }
                  );

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
                    autobrr.serviceConfig = {
                      DynamicUser = lib.mkForce false;
                      User = "autobrr";
                      Group = "autobrr";
                      EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/autobrr-env";
                    };

                    # autobrr has no env/config way to preset an API token — keys live
                    # only in its sqlite DB, created by migrations on first start. Seed
                    # the configuratarr token post-start (the table doesn't exist until
                    # then), ordered before configuratarr's sync.
                    autobrr-apikey-seed = lib.mkIf cfg.configuratarr.enable {
                      description = "Seed the configuratarr API token into autobrr";
                      after = [ "autobrr.service" ];
                      requires = [ "autobrr.service" ];
                      before = [ "configuratarr.service" ];
                      wantedBy = [ "multi-user.target" ];
                      path = [ pkgs.sqlite ];
                      serviceConfig = {
                        Type = "oneshot";
                        RemainAfterExit = true;
                        User = "autobrr";
                        Group = "autobrr";
                        EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/autobrr-env";
                      };
                      script = ''
                        db=/var/lib/autobrr/autobrr.db
                        until sqlite3 "$db" "SELECT 1 FROM api_key LIMIT 1;" >/dev/null 2>&1; do
                          sleep 1
                        done
                        sqlite3 "$db" \
                          "INSERT OR IGNORE INTO api_key (name, key) VALUES ('configuratarr', '$AUTOBRR_API_TOKEN');"
                      '';
                    };

                    configuratarr = lib.mkIf cfg.configuratarr.enable {
                      after = [ "autobrr-apikey-seed.service" ];
                      requires = [ "autobrr-apikey-seed.service" ];
                      serviceConfig.EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/autobrr-env";
                    };
                  };
                };
            };
        };
      };
  };
}
