# modules/hosts/server/containers/zigbee2mqtt.nix
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.zigbee2mqtt = {
    aspect-name = "hosts.server.containers.zigbee2mqtt";
    includes = [
      den.aspects.containers._.zigbee2mqtt
      den.aspects.infra._.clients._.container-zigbee2mqtt
      den.aspects.infra._.clients._.container-mosquitto
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
      let
        cfg = config.containerConfigs.zigbee2mqtt;
      in
      {
        containerConfigs.zigbee2mqtt = {
          lldpd.enable = true;
          hostBridge = "br-iot";
          macAddress = config.infra.clients.container-zigbee2mqtt.mac;
          localAddress = "${config.infra.clients.container-zigbee2mqtt.ip}/${toString config.infra.vlans.iot.subnetPrefix}";
          gateway = config.infra.vlans.iot.staticAddresses.gateway;
          port = config.infra.clients.container-zigbee2mqtt.ports.default;
          idVendor = "303a";
          idProduct = "831a";
          serialPort = "/dev/zigbee";
          mqttHost = config.infra.clients.container-mosquitto.ip;
          mqttPort = config.infra.clients.container-mosquitto.ports.default;
          bindMounts = {
            "/var/lib/zigbee2mqtt" = {
              hostPath = "/var/lib/appdata/zigbee2mqtt";
              isReadOnly = false;
            };
          };
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
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/zigbee2mqtt 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/zigbee2mqtt 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@zigbee2mqtt" = {
          before = [ "container-wave-core.target" ];
          wants = [ "container@mosquitto.service" ];
          after = [
            "container@mosquitto.service"
            "container-wave-foundation.target"
          ];
        };
      };
  };
}
