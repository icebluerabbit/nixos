# modules/infra/clients/container-cinny.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-cinny";
    resourceName = "container_cinny";
    name = "Cinny Container";
    mac = "62:f5:a4:5b:20:74";
    vlan = "apps";
    kind = "virtual";
    ports.default = 80;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-cinny = {
    aspect-name = "infra.clients.container-cinny";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.apps
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.apps.staticAddresses.cinny;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.apps.staticAddresses.cinny;
  };
}
