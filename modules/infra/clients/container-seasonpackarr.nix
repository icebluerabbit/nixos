# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-seasonpackarr";
    resourceName = "container_seasonpackarr";
    name = "seasonpackarr Container";
    mac = "62:f5:a4:5b:20:9a";
    vlan = "media";
    kind = "virtual";
    ports.default = 42069;
  };
in
{
  den.aspects.infra._.clients._.container-seasonpackarr = {
    aspect-name = "infra.clients.container-seasonpackarr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.seasonpackarr;
    terranix =
      { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.seasonpackarr;
  };
}
