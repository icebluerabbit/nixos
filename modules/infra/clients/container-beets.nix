# modules/infra/clients/container-beets.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-beets";
    resourceName = "container_beets";
    name = "Beets Container";
    mac = "62:f5:a4:5b:20:87";
    vlan = "media";
    kind = "virtual";
    ports.default = 8337;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-beets = {
    aspect-name = "infra.clients.container-beets";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.beets;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.beets;
  };
}
