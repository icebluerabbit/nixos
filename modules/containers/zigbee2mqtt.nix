# zigbee2mqtt — Zigbee coordinator bridge via Home Assistant Connect ZBT-2
_: {
  den.aspects.containers._.zigbee2mqtt = {
    aspect-name = "containers.zigbee2mqtt";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        cfg = config.containerConfigs.zigbee2mqtt;
      in
      {
        options.containerConfigs.zigbee2mqtt =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 8080;
            defaultUid = 5021;
            defaultGid = 5021;
          })
          // {
            serialPort = lib.mkOption {
              type = lib.types.str;
              default = "/dev/zigbee";
              description = "Serial device path for ZBT-2 inside the container";
            };
            idVendor = lib.mkOption {
              type = lib.types.str;
              default = "303a";
              description = "USB vendor ID of the Zigbee controller";
            };
            idProduct = lib.mkOption {
              type = lib.types.str;
              default = "831a";
              description = "USB product ID of the Zigbee controller";
            };
            mqttHost = lib.mkOption {
              type = lib.types.str;
              description = "Mosquitto broker IP";
            };
            mqttPort = lib.mkOption {
              type = lib.types.port;
              default = 1883;
              description = "Mosquitto broker port";
            };
          };

        config = {
          containers.zigbee2mqtt = {
            inherit (cfg) autoStart;
            ephemeral = true;
            timeoutStartSec = "5min";
            privateNetwork = true;
            inherit (cfg) hostBridge;
            localMacAddress = cfg.macAddress;
            inherit (cfg) localAddress;
            bindMounts = cfg.bindMounts // {
              "/dev/zigbee" = {
                hostPath = cfg.serialPort;
                isReadOnly = false;
              };
            };

            config =
              { lib, ... }:
              {
                imports = [ (import ./_base.nix { inherit lib cfg; }) ];

                services.zigbee2mqtt = {
                  enable = true;
                  settings = {
                    serial = {
                      port = cfg.serialPort;
                      adapter = "ember";
                      baudrate = 460800;
                    };
                    mqtt.server = "mqtt://${cfg.mqttHost}:${toString cfg.mqttPort}";
                    frontend = {
                      enabled = true;
                      inherit (cfg) port;
                    };
                    homeassistant.enabled = true;
                    availability = true;
                    permit_join = false;
                  };
                };

                users.users.zigbee2mqtt = {
                  uid = lib.mkForce cfg.uid;
                  isSystemUser = lib.mkForce true;
                };
                users.groups.zigbee2mqtt.gid = lib.mkForce cfg.gid;
              };
          };

          systemd.services."container@zigbee2mqtt" = {
            requires = [ "dev-${lib.removePrefix "/dev/" cfg.serialPort}.device" ];
            after = [
              "dev-${lib.removePrefix "/dev/" cfg.serialPort}.device"
            ];
            serviceConfig.DeviceAllow = [ "char-ttyACM rwm" ];
          };

          services.udev.extraRules = ''
            SUBSYSTEM=="tty", ATTRS{idVendor}=="${cfg.idVendor}", ATTRS{idProduct}=="${cfg.idProduct}", SYMLINK+="${lib.removePrefix "/dev/" cfg.serialPort}", TAG+="systemd"
          '';
        };
      };
  };
}
