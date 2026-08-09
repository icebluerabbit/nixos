# _automation.nix — Automation tab: Home Assistant, radios, MQTT, and the
# matrix/chat services. (esphome intentionally omitted.)
{
  cl,
  caddy,
  at,
  promUrl,
  ...
}:
let
  hc =
    {
      name,
      icon,
      href,
      desc,
      monitor ? null,
    }:
    {
      "${name}" = {
        inherit icon href;
        description = desc;
      }
      // (if monitor == null then { } else { siteMonitor = monitor; });
    };
in
[
  {
    "Home Automation" = [
      {
        "Home Assistant" = {
          icon = "home-assistant.svg";
          href = caddy "home-assistant";
          widget = {
            type = "homeassistant";
            url = "http://${cl.container-home-assistant.ip}:${toString cl.container-home-assistant.ports.http}";
            key = "{{HOMEPAGE_VAR_HASS_TOKEN}}";
            fields = [
              "people_home"
              "lights_on"
              "switches_on"
            ];
          };
        };
      }
      {
        "Z-Wave" = {
          icon = "sh-z-wave-js-ui.svg";
          href = caddy "zwave";
          siteMonitor = at cl.container-zwave.ip cl.container-zwave.ports.ui;
          description = "Z-Wave JS UI";
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 30000;
            metrics = [
              {
                label = "Switches On";
                query = "sum(zwave_binary_switch_currentvalue)";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Devices";
                query = "count(zwave_binary_switch_currentvalue)";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
            ];
          };
        };
      }
      (hc {
        name = "Zigbee2MQTT";
        icon = "zigbee2mqtt.svg";
        href = caddy "zigbee2mqtt";
        monitor = at cl.container-zigbee2mqtt.ip cl.container-zigbee2mqtt.ports.default;
        desc = "Zigbee coordinator bridge";
      })
      {
        "Mosquitto" = {
          icon = "mdi-transit-connection-variant-#8be9fd";
          href = caddy "grafana";
          description = "MQTT broker";
          widget = {
            type = "prometheusmetric";
            url = promUrl;
            refreshInterval = 15000;
            metrics = [
              {
                label = "Clients";
                query = "mosquitto_clients_connected";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Msgs/min";
                query = "mosquitto_load_messages_received_1min";
                format = {
                  type = "number";
                  options.maximumFractionDigits = 0;
                };
              }
              {
                label = "Uptime";
                query = "mosquitto_uptime";
                format.type = "duration";
              }
            ];
          };
        };
      }
      (hc {
        name = "Cinny";
        icon = "element.svg";
        href = caddy "cinny";
        monitor = at cl.container-cinny.ip cl.container-cinny.ports.default;
        desc = "Matrix web client";
      })
      (hc {
        name = "TheLounge";
        icon = "thelounge.svg";
        href = caddy "thelounge";
        monitor = at cl.container-thelounge.ip cl.container-thelounge.ports.default;
        desc = "Web IRC client";
      })
      (hc {
        name = "Tuwunel";
        icon = "matrix.svg";
        href = at cl.container-tuwunel.ip cl.container-tuwunel.ports.default;
        monitor = at cl.container-tuwunel.ip cl.container-tuwunel.ports.default;
        desc = "Matrix homeserver";
      })
      (hc {
        name = "Hermes";
        icon = "mdi-robot-#bd93f9";
        href = at cl.container-hermes.ip cl.container-hermes.ports.default;
        # `/` 404s; `/health` answers 200.
        monitor = "${at cl.container-hermes.ip cl.container-hermes.ports.default}/health";
        desc = "Matrix AI bridge bot";
      })
    ];
  }
]
