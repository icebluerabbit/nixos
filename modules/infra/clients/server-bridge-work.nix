# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-bridge-work";
  resourceName = "server_bridge_work";
  name = "Server Work Bridge";
  mac = "82:13:8a:a0:4f:66";
  vlan = "work";
  addressKey = "server";
  dependsOn = [ "server_vlan_work" ];
}
