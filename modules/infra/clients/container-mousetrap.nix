# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-mousetrap";
  resourceName = "container_mousetrap";
  name = "Mousetrap Container";
  mac = "62:f5:a4:5b:20:8a";
  vlan = "media";
  addressKey = "mousetrap";
  ports.default = 8080;
}
