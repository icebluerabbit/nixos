# modules/infra/clients/container-esphome.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-esphome";
    resourceName = "container_esphome";
    name = "ESPHome Container";
    mac = "62:f5:a4:5b:20:20";
    vlan = "iot";
    kind = "virtual";
  };
in
{
  den.aspects.infra._.clients._.container-esphome = {
    aspect-name = "infra.clients.container-esphome";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.iot
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.iot.staticAddresses.esphome;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.iot.staticAddresses.esphome;
  };
}
