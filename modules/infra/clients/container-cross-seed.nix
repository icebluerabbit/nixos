# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-cross-seed";
  resourceName = "container_cross_seed";
  name = "cross-seed Container";
  mac = "62:f5:a4:5b:20:b3";
  vlan = "media";
  addressKey = "cross-seed";
  ports.default = 2468;
  proxy.port = "default";
}
