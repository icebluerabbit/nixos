# esphome — host configuration override for esphome oneshot compiler container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.esphome = {
    aspect-name = "hosts.server.containers.esphome";
    includes = [
      den.aspects.containers._.esphome
      den.aspects.infra._.clients._.container-esphome
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.iot
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.iot
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      {
        systemd.services."container@esphome" = {
          after = [ "container-wave-foundation.target" ];
          before = [ "container-wave-core.target" ];
        };

        containerConfigs.esphome = {

          autoStart = false;
          lldpd.enable = true;
          device = "/var/lib/esphome/device";

          hostBridge = "br-iot";
          macAddress = config.infra.clients.container-esphome.mac;
          localAddress = "${config.infra.clients.container-esphome.ip}/${toString config.infra.vlans.iot.subnetPrefix}";
          gateway = config.infra.vlans.iot.staticAddresses.gateway;
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
          };

          bindMounts = {
            "/var/lib/esphome" = {
              hostPath = "/var/lib/appdata/esphome";
              isReadOnly = false;
            };
            "/run/secrets/esphome-env" = {
              hostPath = config.sops.templates."esphome-env".path;
              isReadOnly = true;
            };
          };

          # Starter Nix object for ESPHome config
          config = {
            esphome = {
              name = "reterminal-e1003";
              friendly_name = "reTerminal ePaper Display";
              comment = "Server and UniFi Stats Display";
              platformio_options = {
                "board_build.flash_mode" = "dio";
              };
              includes = [ "<driver/gpio.h>" ];
              on_boot = {
                priority = 900.0;
                "then" = [
                  {
                    lambda = ''
                      gpio_set_direction(GPIO_NUM_11, GPIO_MODE_OUTPUT);
                      gpio_set_level(GPIO_NUM_11, 1);
                      gpio_set_direction(GPIO_NUM_21, GPIO_MODE_OUTPUT);
                      gpio_set_level(GPIO_NUM_21, 1);
                    '';
                  }
                ];
              };
            };

            # Pull in the IT8951 component from ESPHome PR #15346
            external_components = [
              {
                source = "github://pr#15346";
                components = [ "it8951" ];
              }
            ];

            esp32 = {
              board = "esp32-s3-devkitc-1";
              variant = "esp32s3";
              framework = {
                type = "esp-idf";
              };
            };

            wifi = {
              ssid = "derp";
              password = "$wifi_password";
              manual_ip = {
                static_ip = config.infra.clients.reterminal-e1003.ip;
                gateway = config.infra.vlans.iot.staticAddresses.gateway;
                subnet = "255.255.255.0";
              };
            };

            api = { };
            ota = {
              platform = "esphome";
            };
            logger = { };

            # SPI interface pins for the E1003 display
            spi = {
              clk_pin = "GPIO7";
              mosi_pin = "GPIO9";
              miso_pin = "GPIO8";
            };

            # I2C Bus for the onboard environment sensor
            i2c = {
              sda = "GPIO19";
              scl = "GPIO20";
            };

            font = [
              {
                file = "gfonts://Roboto";
                id = "font_large";
                size = 48;
              }
              {
                file = "gfonts://Roboto";
                id = "font_small";
                size = 24;
              }
            ];

            sensor = [
              {
                platform = "sht4x";
                temperature = {
                  name = "Ambient Temperature";
                  id = "ambient_temp";
                };
                humidity = {
                  name = "Ambient Humidity";
                  id = "ambient_hum";
                };
                address = 68;
                update_interval = "60s";
              }
            ];

            display = [
              {
                platform = "it8951";
                model = "seeed-reterminal-e1003";
                id = "e1003_display";
                cs_pin = "GPIO10";
                reset_pin = "GPIO12";
                busy_pin = "GPIO13";
                vcom = 1580;
                dimensions = {
                  width = 1404;
                  height = 1872;
                };
                rotation = "0°";
                update_interval = "60s";
                lambda = ''
                  // Draw a sleek frame border around the screen edge
                  it.rectangle(10, 10, 1384, 1852, Color::BLACK);

                  // Print "Hello World" perfectly centered horizontally
                  it.print(702, 400, id(font_large), Color::BLACK, TextAlign::CENTER, "Hello World");

                  // Draw a subtle horizontal divider line under the headline
                  it.line(400, 480, 1004, 480, Color::BLACK);

                  // Print the local environment statistics safely
                  if (id(ambient_temp).has_state()) {
                    it.printf(702, 540, id(font_small), Color::BLACK, TextAlign::CENTER,
                              "System Temp: %.1f°C  |  Humidity: %.1f%%",
                              id(ambient_temp).state, id(ambient_hum).state);
                  } else {
                    it.print(702, 540, id(font_small), Color::BLACK, TextAlign::CENTER,
                             "Sensor Initializing...");
                  }
                '';
              }
            ];
          };
        };

        # Host-level directory setup for PlatformIO persistent cache
        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/esphome 0755 root root - -"
        ];
      };
  };
}
