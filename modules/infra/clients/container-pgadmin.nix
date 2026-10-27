# modules/infra/clients/container-pgadmin.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-pgadmin";
    resourceName = "container_pgadmin";
    name = "pgAdmin Container";
    mac = "62:f5:a4:5b:20:8e";
    vlan = "services";
    kind = "virtual";
    ports.default = 5050;
    proxy.port = "default";
  };
in
{
  den.aspects.infra._.clients._.container-pgadmin = {
    aspect-name = "infra.clients.container-pgadmin";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.services
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.services.staticAddresses.pgadmin;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.services.staticAddresses.pgadmin;
  };
}
