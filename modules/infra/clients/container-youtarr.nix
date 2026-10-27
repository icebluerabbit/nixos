# modules/infra/clients/container-youtarr.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-youtarr";
    resourceName = "container_youtarr";
    name = "Youtarr Container";
    mac = "62:f5:a4:5b:20:7e";
    vlan = "media";
    kind = "virtual";
    ports.http = 3011;
    proxy.port = "http";
  };
in
{
  den.aspects.infra._.clients._.container-youtarr = {
    aspect-name = "infra.clients.container-youtarr";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.youtarr;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.youtarr;
  };
}
