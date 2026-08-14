# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-bridge-storage";
  resourceName = "server_bridge_storage";
  name = "Server Storage Bridge";
  mac = "82:13:8a:a0:4f:52";
  vlan = "storage";
  addressKey = "server";
  dependsOn = [ "server_vlan_storage" ];
}
