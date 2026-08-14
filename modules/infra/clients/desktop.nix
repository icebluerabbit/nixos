# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "desktop";
  resourceName = "desktop";
  name = "Desktop";
  mac = "b0:82:e2:32:f9:94";
  vlan = "trust";
  addressKey = "desktop";
}
