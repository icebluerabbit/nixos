# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-cross-seed";
    resourceName = "container_cross_seed";
    name = "cross-seed Container";
    mac = "62:f5:a4:5b:20:b3";
    vlan = "media";
    kind = "virtual";
    ports.default = 2468;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-cross-seed = {
    aspect-name = "infra.clients.container-cross-seed";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.cross-seed;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.cross-seed;
  };
}
