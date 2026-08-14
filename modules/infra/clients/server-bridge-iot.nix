# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-bridge-iot";
  resourceName = "server_bridge_iot";
  name = "Server IoT Bridge";
  mac = "82:13:8a:a0:4f:3e";
  vlan = "iot";
  addressKey = "server";
  dependsOn = [ "server_vlan_iot" ];
}
