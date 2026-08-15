# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-flaresolverr";
    resourceName = "container_flaresolverr";
    name = "FlareSolverr Container";
    mac = "62:f5:a4:5b:20:83";
    vlan = "media";
    kind = "virtual";
    ports.default = 8191;
  };
in
{
  den.aspects.infra._.clients._.container-flaresolverr = {
    aspect-name = "infra.clients.container-flaresolverr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.flaresolverr;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.flaresolverr;
  };
}
