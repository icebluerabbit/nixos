# modules/infra/clients/container-pgadmin.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-pgadmin";
  resourceName = "container_pgadmin";
  name = "pgAdmin Container";
  mac = "62:f5:a4:5b:20:8e";
  vlan = "services";
  addressKey = "pgadmin";
  ports.default = 5050;
  proxy.port = "default";
}
