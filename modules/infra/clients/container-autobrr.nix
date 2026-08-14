# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-autobrr";
  resourceName = "container_autobrr";
  name = "Autobrr Container";
  mac = "62:f5:a4:5b:20:b2";
  vlan = "media";
  addressKey = "autobrr";
  ports.default = 7474;
  proxy.port = "default";
}
