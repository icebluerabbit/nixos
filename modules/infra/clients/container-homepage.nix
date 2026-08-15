# modules/infra/clients/container-homepage.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-homepage";
    resourceName = "container_homepage";
    name = "Homepage Container";
    mac = "62:f5:a4:5b:20:71";
    vlan = "apps";
    kind = "virtual";
    ports.default = 8082;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-homepage = {
    aspect-name = "infra.clients.container-homepage";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.apps
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.apps.staticAddresses.homepage;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.apps.staticAddresses.homepage;
  };
}
