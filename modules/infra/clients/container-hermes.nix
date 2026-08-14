# modules/infra/clients/container-hermes.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-hermes";
  resourceName = "container_hermes";
  name = "Hermes Agent Container";
  mac = "62:f5:a4:5b:20:99";
  vlan = "services";
  addressKey = "hermes";
  ports.default = 8642;
  proxy.port = "default";
}
