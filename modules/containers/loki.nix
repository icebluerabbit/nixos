# modules/containers/loki.nix
_: {
  den.aspects.containers._.loki = {
    aspect-name = "containers.loki";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      {
        options.containerConfigs.loki =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 3100;
            defaultUid = 5100;
            defaultGid = 5100;
          })
          // {
            grpcPort = lib.mkOption {
              type = lib.types.port;
              default = 9096;
            };
          };

        config = {
          systemd.services."container@loki" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.loki.memoryHigh != null
            ) config.containerConfigs.loki.memoryHigh;
          };

          containers.loki =
            let
              cfg = config.containerConfigs.loki;
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
                { ... }:
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                  ];

                  users.users.loki = {
                    uid = lib.mkForce cfg.uid;
                    group = "loki";
                    isSystemUser = true;
                  };
                  users.groups.loki.gid = lib.mkForce cfg.gid;

                  services.loki = {
                    enable = true;
                    configFile = null;
                    configuration = {
                      auth_enabled = false;
                      server = {
                        http_listen_port = cfg.port;
                        grpc_listen_port = cfg.grpcPort;
                      };
                      common = {
                        ring = {
                          instance_addr = "127.0.0.1";
                          kvstore = {
                            store = "inmemory";
                          };
                        };
                        replication_factor = 1;
                        path_prefix = "/var/lib/loki";
                      };
                      schema_config = {
                        configs = [
                          {
                            from = "2020-10-24";
                            store = "tsdb";
                            object_store = "filesystem";
                            schema = "v13";
                            index = {
                              prefix = "index_";
                              period = "24h";
                            };
                          }
                        ];
                      };
                      storage_config = {
                        filesystem = {
                          directory = "/var/lib/loki/chunks";
                        };
                      };
                      pattern_ingester = {
                        enabled = true;
                      };
                    };
                  };

                };
            };
        };
      };
  };
}
