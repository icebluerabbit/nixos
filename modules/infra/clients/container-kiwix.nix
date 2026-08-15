# modules/infra/clients/container-kiwix.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-kiwix";
    resourceName = "container_kiwix";
    name = "Kiwix Container";
    mac = "62:f5:a4:5b:20:72";
    vlan = "apps";
    kind = "virtual";
    ports.http = 8080;
    proxy.port = "http";
  };
in
{
  den.aspects.infra._.clients._.container-kiwix = {
    aspect-name = "infra.clients.container-kiwix";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.apps
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.apps.staticAddresses.kiwix;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.apps.staticAddresses.kiwix;
  };
}
