# modules/infra/clients/container-homepage.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-homepage";
  resourceName = "container_homepage";
  name = "Homepage Container";
  mac = "62:f5:a4:5b:20:71";
  vlan = "apps";
  addressKey = "homepage";
  ports.default = 8082;
  proxy.port = "default";
}
