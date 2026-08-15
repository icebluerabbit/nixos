# modules/infra/clients/container-home-assistant.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-home-assistant";
    resourceName = "container_home_assistant";
    name = "Home Assistant Container";
    mac = "62:f5:a4:5b:20:1f";
    vlan = "iot";
    kind = "virtual";
    ports.http = 8123;
    proxy.port = "http";
  };
in
{
  den.aspects.infra._.clients._.container-home-assistant = {
    aspect-name = "infra.clients.container-home-assistant";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.iot
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.iot.staticAddresses.home-assistant;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.iot.staticAddresses.home-assistant;
  };
}
