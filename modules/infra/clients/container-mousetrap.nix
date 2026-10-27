# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-mousetrap";
    resourceName = "container_mousetrap";
    name = "Mousetrap Container";
    mac = "62:f5:a4:5b:20:8a";
    vlan = "media";
    kind = "virtual";
    ports.default = 8080;
  };
in
{
  den.aspects.infra._.clients._.container-mousetrap = {
    aspect-name = "infra.clients.container-mousetrap";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.mousetrap;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.mousetrap;
  };
}
