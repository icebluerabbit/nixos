# modules/infra/clients/container-komga.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-komga";
    resourceName = "container_komga";
    name = "Komga Container";
    mac = "62:f5:a4:5b:20:b6";
    vlan = "media";
    kind = "virtual";
    ports.default = 25600;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-komga = {
    aspect-name = "infra.clients.container-komga";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.komga;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.komga;
  };
}
