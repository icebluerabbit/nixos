# modules/infra/clients/container-zigbee2mqtt.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-zigbee2mqtt";
    resourceName = "container_zigbee2mqtt";
    name = "Zigbee2MQTT Container";
    mac = "62:f5:a4:5b:20:23";
    vlan = "iot";
    kind = "virtual";
    ports.default = 8080;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-zigbee2mqtt = {
    aspect-name = "infra.clients.container-zigbee2mqtt";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.iot
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.iot.staticAddresses.zigbee2mqtt;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.iot.staticAddresses.zigbee2mqtt;
  };
}
