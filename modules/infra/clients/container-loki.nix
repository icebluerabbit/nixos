# modules/infra/clients/container-loki.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-loki";
    resourceName = "container_loki";
    name = "Loki Container";
    mac = "62:f5:a4:5b:20:94";
    vlan = "services";
    kind = "virtual";
    ports.default = 3100;
  };
in
{
  den.aspects.infra._.clients._.container-loki = {
    aspect-name = "infra.clients.container-loki";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.services
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.services.staticAddresses.loki;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.services.staticAddresses.loki;
  };
}
