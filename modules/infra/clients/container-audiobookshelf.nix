# modules/infra/clients/container-audiobookshelf.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-audiobookshelf";
    resourceName = "container_audiobookshelf";
    name = "Audiobookshelf Container";
    mac = "62:f5:a4:5b:20:b7";
    vlan = "media";
    kind = "virtual";
    ports.default = 13378;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-audiobookshelf = {
    aspect-name = "infra.clients.container-audiobookshelf";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.audiobookshelf;
    terranix =
      { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.audiobookshelf;
  };
}
