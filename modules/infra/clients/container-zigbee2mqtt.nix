# modules/infra/clients/container-zigbee2mqtt.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-zigbee2mqtt";
  resourceName = "container_zigbee2mqtt";
  name = "Zigbee2MQTT Container";
  mac = "62:f5:a4:5b:20:23";
  vlan = "iot";
  addressKey = "zigbee2mqtt";
  ports.default = 8080;
  proxy.port = "default";
}
