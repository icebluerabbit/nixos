# modules/containers/unifi-exporter.nix
_: {
  den.aspects.containers._.unifi-exporter = {
    aspect-name = "containers.unifi-exporter";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
      in
      {
        options.containerConfigs.unifi-exporter =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 9130;
            defaultUid = 5400;
            defaultGid = 5400;
          })
          // {
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to the environment file containing secrets to be mounted and loaded";
            };
            syslogPort = lib.mkOption {
              type = lib.types.port;
              description = "UDP port to accept incoming syslog traffic from UniFi devices";
            };
            controllerUrl = lib.mkOption {
              type = lib.types.str;
              description = "URL of the UniFi Controller";
            };
          };

        config = {
          systemd.services."container@unifi-exporter" = {
            serviceConfig.MemoryHigh = lib.mkIf (
              config.containerConfigs.unifi-exporter.memoryHigh != null
            ) config.containerConfigs.unifi-exporter.memoryHigh;
          };

          containers.unifi-exporter =
            let
              cfg = hostConfig.containerConfigs.unifi-exporter;
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
                  "/run/secrets/unpoller-env" = {
                    hostPath = cfg.envFile;
                    isReadOnly = true;
                  };
                });

              config =
                {
                  lib,
                  ...
                }:
                {
                  imports = [
                    (import ./_base.nix { inherit lib cfg; })
                  ];

                  services.unpoller = {
                    enable = true;
                    poller.quiet = true;
                    prometheus.http_listen = "[::]:${toString cfg.port}";
                    influxdb.disable = true;
                    unifi.defaults = {
                      url = cfg.controllerUrl;
                      verify_ssl = false;
                    };
                  };

                  systemd.services.unifi-poller.serviceConfig = lib.mkIf (cfg.envFile != null) {
                    EnvironmentFile = "/run/secrets/unpoller-env";
                  };

                  # Open UDP port for incoming syslog traffic from UniFi devices
                  networking.firewall.allowedUDPPorts = [ cfg.syslogPort ];
                };
            };
        };
      };
  };
}
