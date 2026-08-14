# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-bridge-guest";
  resourceName = "server_bridge_guest";
  name = "Server Guest Bridge";
  mac = "82:13:8a:a0:4f:48";
  vlan = "guest";
  addressKey = "server";
  dependsOn = [ "server_vlan_guest" ];
}
