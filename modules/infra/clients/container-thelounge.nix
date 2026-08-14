# modules/infra/clients/container-thelounge.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-thelounge";
  resourceName = "container_thelounge";
  name = "TheLounge Container";
  mac = "62:f5:a4:5b:20:75";
  vlan = "apps";
  addressKey = "thelounge";
  ports.default = 9000;
  proxy.port = "default";
}
