# modules/infra/clients/container-unpackerr.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-unpackerr";
    resourceName = "container_unpackerr";
    name = "Unpackerr Container";
    mac = "62:f5:a4:5b:20:81";
    vlan = "media";
    kind = "virtual";
    ports.metrics = 5656;
  };
in
{
  den.aspects.infra._.clients._.container-unpackerr = {
    aspect-name = "infra.clients.container-unpackerr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.unpackerr;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.unpackerr;
  };
}
