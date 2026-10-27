# modules/infra/clients/container-mosquitto.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-mosquitto";
    resourceName = "container_mosquitto";
    name = "Mosquitto Container";
    mac = "62:f5:a4:5b:20:22";
    vlan = "iot";
    kind = "virtual";
    ports.default = 1883;
  };
in
{
  den.aspects.infra._.clients._.container-mosquitto = {
    aspect-name = "infra.clients.container-mosquitto";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.iot
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.iot.staticAddresses.mosquitto;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.iot.staticAddresses.mosquitto;
  };
}
