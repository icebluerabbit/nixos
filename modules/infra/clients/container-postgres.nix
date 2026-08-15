# modules/infra/clients/container-postgres.nix
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "container-postgres";
    resourceName = "container_postgres";
    name = "PostgreSQL Container";
    mac = "62:f5:a4:5b:20:8d";
    vlan = "services";
    kind = "virtual";
    ports = {
      default = 5432;
      exporter = 9187;
    };
  };
in
{
  den.aspects.infra._.clients._.container-postgres = {
    aspect-name = "infra.clients.container-postgres";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.services
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.services.staticAddresses.postgres;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.services.staticAddresses.postgres;
  };
}
