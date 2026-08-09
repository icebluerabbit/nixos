# zwave — Z-Wave JS UI server for Z-Wave network management
_: {
  den.aspects.containers._.zwave = {
    aspect-name = "containers.zwave";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        hostConfig = config;
        cfg = hostConfig.containerConfigs.zwave;
      in
      {
        options.containerConfigs.zwave =
          (import ./_options.nix {
            inherit lib;
            defaultUid = 5014;
            defaultGid = 5014;
          })
          // {
            uiPort = lib.mkOption {
              type = lib.types.port;
              default = 8091;
              description = "Z-Wave JS UI web interface port";
            };
            wsPort = lib.mkOption {
              type = lib.types.port;
              default = 3000;
              description = "Z-Wave JS WebSocket server port";
            };
            serialPort = lib.mkOption {
              type = lib.types.str;
              default = "/dev/zwave";
              description = "Serial device path for the Z-Wave controller inside the container";
            };
            idVendor = lib.mkOption {
              type = lib.types.str;
              default = "303a";
              description = "USB vendor ID of the Z-Wave controller";
            };
            idProduct = lib.mkOption {
              type = lib.types.str;
              default = "4001";
              description = "USB product ID of the Z-Wave controller";
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = "Host path to environment file containing Z-Wave security keys";
            };
          };

        config = {
          containers.zwave = {
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
                "/run/secrets/zwave-env" = {
                  hostPath = cfg.envFile;
                  isReadOnly = true;
                };
              });
            allowedDevices = [ ];

            config =
              { config, lib, ... }:
              {
                imports = [
                  (import ./_base.nix {
                    inherit lib cfg;
                    hasPort = false;
                  })
                ];

                networking.firewall.allowedTCPPorts = [
                  cfg.uiPort
                  cfg.wsPort
                ];

                services.zwave-js-ui = {
                  enable = true;
                  inherit (cfg) serialPort;
                };

                systemd.services.zwave-js-ui.environment = {
                  TRUST_PROXY = "1";
                };

                systemd.services.zwave-js-ui.serviceConfig = {
                  DynamicUser = lib.mkForce false;
                  User = lib.mkForce config.users.users.zwave-js-ui.name;
                  Group = lib.mkForce config.users.groups.zwave-js-ui.name;
                  EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/zwave-env";
                };

                users.users.zwave-js-ui = {
                  uid = lib.mkForce cfg.uid;
                  isSystemUser = lib.mkForce false;
                  isNormalUser = true;
                };
                users.groups.zwave-js-ui.gid = lib.mkForce cfg.gid;
              };
          };

          systemd.services."container@zwave" = {
            requires = [ "dev-${lib.removePrefix "/dev/" cfg.serialPort}.device" ];
            after = [
              "dev-${lib.removePrefix "/dev/" cfg.serialPort}.device"
            ];
            serviceConfig.DeviceAllow = [ "char-ttyACM rwm" ];
            serviceConfig.MemoryHigh = lib.mkIf (cfg.memoryHigh != null) cfg.memoryHigh;
          };

          services.udev.extraRules = ''
            SUBSYSTEM=="tty", ATTRS{idVendor}=="${cfg.idVendor}", ATTRS{idProduct}=="${cfg.idProduct}", SYMLINK+="${lib.removePrefix "/dev/" cfg.serialPort}", TAG+="systemd"
          '';
        };
      };
  };
}
