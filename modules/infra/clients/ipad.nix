# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "ipad";
  resourceName = "ipad";
  name = "iPad";
  mac = "4e:37:93:b9:e0:6a";
  ip = "10.10.20.50";
}
