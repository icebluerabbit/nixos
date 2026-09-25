# esphome — declarative offline esphome oneshot compiler and flasher
_: {
  den.aspects.containers._.esphome = {
    aspect-name = "containers.esphome";
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
        cfg = hostConfig.containerConfigs.esphome;
      in
      {
        options.containerConfigs.esphome = (import ./_options.nix { inherit lib; }) // {
          device = lib.mkOption {
            type = lib.types.str;
            default = "/var/lib/esphome/device";
            description = "Serial device path inside the container";
          };
          hostDevicePath = lib.mkOption {
            type = lib.types.str;
            default = "/var/lib/appdata/esphome/device";
            description = "The host-side path where udev will create the device node";
          };
          idVendor = lib.mkOption {
            type = lib.types.str;
            default = "1a86";
            description = "USB Vendor ID of the target flashing device";
          };
          idProduct = lib.mkOption {
            type = lib.types.str;
            default = "7522";
            description = "USB Product ID of the target flashing device";
          };
          config = lib.mkOption {
            type = lib.types.attrs;
            default = { };
            description = "ESPHome configuration attribute set";
          };
        };

        config = {
          containers.esphome = {
            inherit (cfg) autoStart;
            privateNetwork = true;
            inherit (cfg) hostBridge;
            localMacAddress = cfg.macAddress;
            inherit (cfg) localAddress;
            inherit (cfg) bindMounts;
            allowedDevices = [ ];

            config =
              { pkgs, lib, ... }:
              let
                yamlFormat = pkgs.formats.yaml { };
                esphomeConfigTemplateYaml = yamlFormat.generate "esphome-config.yaml" cfg.config;

                # esphome 2026.8+ installs its own ESP-IDF toolchain at
                # runtime, downloading FHS-linked binaries (cmake, ninja,
                # xtensa-esp-elf, openocd) that cannot exec on NixOS. Run it
                # inside an FHS sandbox so those binaries find ld-linux and
                # their shared libraries.
                esphomeFhs = pkgs.buildFHSEnv {
                  name = "esphome-fhs";
                  targetPkgs = p: [
                    p.esphome
                    p.git
                    p.libusb1
                    p.zlib
                    p.ncurses
                    p.stdenv.cc.cc.lib
                  ];
                  runScript = "esphome";
                  # buildFHSEnv auto-binds top-level host dirs with plain
                  # `bwrap --bind`, which sets MS_NODEV, so the serial node
                  # under /var/lib/esphome cannot be opened. Re-bind it with
                  # device access. The -try variant keeps the sandbox usable
                  # when the board is unplugged and the node is absent.
                  extraBwrapArgs = [
                    "--dev-bind-try"
                    cfg.device
                    cfg.device
                  ];
                };
              in
              {
                imports = [
                  (import ./_base.nix {
                    inherit lib cfg;
                    hasPort = false;
                  })
                ];

                # Install esphome package
                environment.systemPackages = [
                  esphomeFhs
                  pkgs.git
                ];

                systemd = {
                  # Native oneshot service to perform a single check and flash
                  services.esphome-flash = {
                    description = "ESPHome Flasher";
                    serviceConfig = {
                      Type = "oneshot";
                      EnvironmentFile = "/run/secrets/esphome-env";
                      ExecStart = pkgs.writeShellScript "esphome-flash-wrapper" ''
                        CURRENT_CONFIG="${esphomeConfigTemplateYaml}"
                        LAST_FLASHED="/var/lib/esphome/last_flashed_config"

                        echo "ESPHome flasher service started. Monitoring ${cfg.device}..."

                        if [ -e "${cfg.device}" ]; then
                          if [ ! -f "$LAST_FLASHED" ] || [ "$(cat "$LAST_FLASHED")" != "$CURRENT_CONFIG" ]; then
                            echo "ESPHome configuration is out-of-date or not flashed yet. Starting compilation & flash..."
                            ${pkgs.coreutils}/bin/cp "$CURRENT_CONFIG" /var/lib/esphome/esphome-config.yaml
                            if ${esphomeFhs}/bin/esphome-fhs -s wifi_password "$WIFI_PASSWORD" run /var/lib/esphome/esphome-config.yaml --device ${cfg.device} --no-logs; then
                              echo "$CURRENT_CONFIG" > "$LAST_FLASHED"
                              echo "ESPHome flashing completed successfully."
                            else
                              echo "ESPHome flashing failed!" >&2
                              exit 1
                            fi
                          fi
                        fi
                      '';
                      Environment = [
                        "HOME=/var/lib/esphome"
                      ];
                    };
                  };

                  # Companion timer to schedule the oneshot checker
                  timers.esphome-flash = {
                    description = "Trigger ESPHome Flasher periodically";
                    wantedBy = [ "timers.target" ];
                    timerConfig = {
                      OnBootSec = "5s";
                      OnUnitInactiveSec = "5m";
                      AccuracySec = "1s";
                    };
                  };

                  # Ensure /var/lib/esphome exists inside the container
                  tmpfiles.rules = [
                    "d /var/lib/esphome 0755 root root - -"
                  ];
                };

              };
          };

          # Allow USB-to-serial device classes dynamically inside the container's cgroups
          systemd.services."container@esphome" = {
            serviceConfig.DeviceAllow = [
              "char-ttyUSB rwm"
              "char-ttyACM rwm"
            ];
            serviceConfig.MemoryHigh = lib.mkIf (cfg.memoryHigh != null) cfg.memoryHigh;
          };

          # Create/remove the device node dynamically inside the existing appdata directory
          services.udev.extraRules = ''
            ACTION=="add", SUBSYSTEM=="tty", ATTRS{idVendor}=="${cfg.idVendor}", ATTRS{idProduct}=="${cfg.idProduct}", RUN+="${pkgs.coreutils}/bin/mknod ${cfg.hostDevicePath} c %M %m", RUN+="${pkgs.coreutils}/bin/chmod 666 ${cfg.hostDevicePath}"
            ACTION=="remove", SUBSYSTEM=="tty", ENV{ID_VENDOR_ID}=="${cfg.idVendor}", ENV{ID_MODEL_ID}=="${cfg.idProduct}", RUN+="${pkgs.coreutils}/bin/rm -f ${cfg.hostDevicePath}"
          '';
        };
      };
  };
}
