# modules/infra/clients/container-postgres.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-postgres";
  resourceName = "container_postgres";
  name = "PostgreSQL Container";
  mac = "62:f5:a4:5b:20:8d";
  vlan = "services";
  addressKey = "postgres";
  ports = {
    default = 5432;
    exporter = 9187;
  };
}
