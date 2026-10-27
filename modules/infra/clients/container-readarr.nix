# modules/infra/clients/container-readarr.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-readarr";
    resourceName = "container_readarr";
    name = "Readarr Container";
    mac = "62:f5:a4:5b:20:7b";
    vlan = "media";
    kind = "virtual";
    ports.default = 8787;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-readarr = {
    aspect-name = "infra.clients.container-readarr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.readarr;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.readarr;
  };
}
