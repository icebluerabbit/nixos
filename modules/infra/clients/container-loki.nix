# modules/infra/clients/container-loki.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-loki";
  resourceName = "container_loki";
  name = "Loki Container";
  mac = "62:f5:a4:5b:20:94";
  vlan = "services";
  addressKey = "loki";
  ports.default = 3100;
}
