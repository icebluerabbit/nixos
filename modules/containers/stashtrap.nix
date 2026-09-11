# stashtrap — autobrr's external filter for BitPorn announces
# packages/go/stashtrap
#
# autobrr asks it about each release and reads the answer from the status code;
# autobrr itself does the grab. Phase one records every announce and accepts
# none, so the filter can be enabled while the matching is still being built.
_: {
  den.aspects.containers._.stashtrap = {
    aspect-name = "containers.stashtrap";
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
        # Resolved from HOST pkgs: the container is built from its own nixpkgs
        # instance, which carries no homelab overlay. The store path reaches
        # the container's closure through ExecStart.
        stashtrapExe = lib.getExe pkgs.homelab.stashtrap;
      in
      {
        options.containerConfigs.stashtrap =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 8080;
            defaultUid = 5043;
            defaultGid = 5043;
          })
          // {
            metricsPort = lib.mkOption {
              type = lib.types.port;
              default = 9090;
              description = "Loopback port for the Prometheus endpoint the Vector sidecar scrapes";
            };
            dbPath = lib.mkOption {
              type = lib.types.str;
              default = "/var/lib/stashtrap/stashtrap.db";
              description = "SQLite database path; must be local disk, never NFS";
            };
            stashboxEndpoint = lib.mkOption {
              type = lib.types.str;
              default = "https://stashdb.org/graphql";
              description = "stash-box GraphQL endpoint whose favourites form the catalogue";
            };
            stashboxKeyFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to a file holding the stash-box API key; null disables the catalogue and POST /sync";
            };
            syncInterval = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "systemd OnCalendar for the catalogue sync timer; null disables it";
            };
          };

        config = {
          systemd.services."container@stashtrap" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.stashtrap.memoryHigh != null
            ) config.containerConfigs.stashtrap.memoryHigh;
          };

          containers.stashtrap =
            let
              cfg = hostConfig.containerConfigs.stashtrap;
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
                // (lib.optionalAttrs (cfg.stashboxKeyFile != null) {
                  "/run/secrets/stashbox-key" = {
                    hostPath = cfg.stashboxKeyFile;
                    isReadOnly = true;
                  };
                });

              config =
                { pkgs, lib, ... }:
                {
                  imports = [ (import ./_base.nix { inherit lib cfg; }) ];

                  systemd = {
                    services.stashtrap = {
                      description = "Record autobrr announces and decide whether to grab them";
                      wantedBy = [ "multi-user.target" ];
                      after = [ "network.target" ];
                      serviceConfig = {
                        Type = "simple";
                        User = "stashtrap";
                        Group = "stashtrap";
                        ExecStart = lib.concatStringsSep " " (
                          [
                            stashtrapExe
                            "--addr :${toString cfg.port}"
                            "--db-path ${cfg.dbPath}"
                            "--metrics.addr 127.0.0.1:${toString cfg.metricsPort}"
                          ]
                          ++ lib.optionals (cfg.stashboxKeyFile != null) [
                            "--stashbox-endpoint ${cfg.stashboxEndpoint}"
                            "--stashbox-key-file /run/secrets/stashbox-key"
                          ]
                        );
                        Restart = "on-failure";
                        RestartSec = 5;
                      };
                    };

                    # The crawl is a timer rather than a ticker inside the
                    # process: a first fill runs for minutes and has nothing to
                    # do with answering autobrr inside a request timeout. The
                    # endpoint returns 202 immediately and 409 if a crawl is
                    # already running, so an overlapping tick is a no-op.
                    services.stashtrap-sync = lib.mkIf (cfg.syncInterval != null) {
                      description = "Fill the stashtrap stash-box catalogue";
                      after = [ "stashtrap.service" ];
                      requires = [ "stashtrap.service" ];
                      path = [ pkgs.curl ];
                      serviceConfig = {
                        Type = "oneshot";
                        User = "stashtrap";
                        Group = "stashtrap";
                      };
                      script = ''
                        curl -sf -X POST http://127.0.0.1:${toString cfg.port}/sync
                      '';
                    };

                    timers.stashtrap-sync = lib.mkIf (cfg.syncInterval != null) {
                      description = "Periodic stashtrap catalogue sync";
                      wantedBy = [ "timers.target" ];
                      timerConfig = {
                        OnCalendar = cfg.syncInterval;
                        Persistent = true;
                        RandomizedDelaySec = "5m";
                      };
                    };
                  };

                  users.users.stashtrap = {
                    uid = lib.mkForce cfg.uid;
                    group = "stashtrap";
                    isSystemUser = true;
                  };
                  users.groups.stashtrap.gid = lib.mkForce cfg.gid;
                };
            };
        };
      };
  };
}
