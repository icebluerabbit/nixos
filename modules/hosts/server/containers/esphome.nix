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

          autoStart = true;
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
            };

            esp32 = {
              board = "esp32-s3-devkitc-1";
              variant = "esp32s3";
              # Grafana's /render takes ~5s to produce the PNG, and
              # online_image blocks the main loop waiting for the response;
              # the default 5s task watchdog panics mid-request.
              watchdog_timeout = "30s";
              framework = {
                type = "esp-idf";
              };
            };

            # The E1003 carries an ESP32-S3R8: 8 MB octal SPI PSRAM. The
            # it8951 framebuffer is 1314144 bytes (1872x1404 at 4bpp), far
            # beyond internal DRAM, so PSRAM must be initialized or the
            # display fails allocation at setup.
            psram = {
              mode = "octal";
              speed = "80MHz";
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

            http_request = {
              timeout = "30s";
            };

            # Grafana renders the dashboard to a PNG server-side; the panel
            # only redraws when a new image finishes downloading.
            image = [
              {
                id = "dash";
                platform = "online_image";
                url = "http://${config.infra.clients.container-grafana.ip}:${toString config.infra.clients.container-grafana.ports.default}/render/d/epaper-server/epaper-server?orgId=1&width=1000&height=750&scale=1.872&theme=light&kiosk&_dash.hideTimePicker";
                format = "PNG";
                type = "GRAYSCALE";
                update_interval = "10min";
                on_download_finished = [
                  { "component.update" = "e1003_display"; }
                ];
              }
            ];

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
                update_interval = "never";
                lambda = ''
                  it.image(0, 0, id(dash));
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
