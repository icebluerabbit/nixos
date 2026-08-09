# modules/containers/tempo.nix
_: {
  den.aspects.containers._.tempo = {
    aspect-name = "containers.tempo";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      {
        options.containerConfigs.tempo =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 3200;
            defaultUid = 5500;
            defaultGid = 5500;
          })
          // {
            otlpGrpcPort = lib.mkOption {
              type = lib.types.port;
              default = 4317;
              description = "Tempo OTLP gRPC port";
            };
            otlpHttpPort = lib.mkOption {
              type = lib.types.port;
              default = 4318;
              description = "Tempo OTLP HTTP port";
            };
          };

        config = {
          systemd.services."container@tempo" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.tempo.memoryHigh != null
            ) config.containerConfigs.tempo.memoryHigh;
          };

          containers.tempo =
            let
              cfg = config.containerConfigs.tempo;
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
                  lib,
                  config,
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
                    cfg.port
                    cfg.otlpGrpcPort
                    cfg.otlpHttpPort
                  ];

                  users.users.tempo = {
                    uid = lib.mkForce cfg.uid;
                    group = "tempo";
                    isSystemUser = true;
                  };
                  users.groups.tempo.gid = lib.mkForce cfg.gid;

                  systemd.services.tempo.serviceConfig = {
                    DynamicUser = lib.mkForce false;
                    User = config.users.users.tempo.name;
                    Group = config.users.groups.tempo.name;
                  };

                  services.tempo = {
                    enable = true;
                    settings = {
                      stream_over_http_enabled = true;
                      server = {
                        http_listen_port = cfg.port;
                      };
                      distributor = {
                        receivers = {
                          otlp = {
                            protocols = {
                              grpc = {
                                endpoint = "0.0.0.0:${toString cfg.otlpGrpcPort}";
                              };
                              http = {
                                endpoint = "0.0.0.0:${toString cfg.otlpHttpPort}";
                              };
                            };
                          };
                        };
                      };
                      live_store = {
                        max_trace_idle = "10s";
                        max_block_duration = "5m";
                        shutdown_marker_dir = "/var/lib/tempo/live-store/shutdown-marker";
                        wal = {
                          path = "/var/lib/tempo/live-store/traces";
                        };
                      };
                      block_builder = {
                        wal = {
                          path = "/var/lib/tempo/block-builder/traces";
                        };
                      };
                      backend_scheduler = {
                        local_work_path = "/var/lib/tempo/scheduler";
                        provider = {
                          compaction = {
                            compaction = {
                              compaction_window = "1h";
                              max_block_bytes = 100000000;
                              block_retention = "48h";
                            };
                          };
                        };
                      };
                      storage = {
                        trace = {
                          backend = "local";
                          wal = {
                            path = "/var/lib/tempo/wal";
                          };
                          local = {
                            path = "/var/lib/tempo/blocks";
                          };
                        };
                      };
                    };
                  };
                };
            };
        };
      };
  };
}
