# modules/infra/clients/container-beets.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-beets";
  resourceName = "container_beets";
  name = "Beets Container";
  mac = "62:f5:a4:5b:20:87";
  vlan = "media";
  addressKey = "beets";
  ports.default = 8337;
  proxy.port = "default";
}
