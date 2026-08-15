# modules/infra/clients/container-lazylibrarian.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-lazylibrarian";
    resourceName = "container_lazylibrarian";
    name = "LazyLibrarian Container";
    mac = "62:f5:a4:5b:20:80";
    vlan = "media";
    kind = "virtual";
    ports.default = 5299;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-lazylibrarian = {
    aspect-name = "infra.clients.container-lazylibrarian";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.lazylibrarian;
    terranix =
      { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.lazylibrarian;
  };
}
