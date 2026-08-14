# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-bridge-dmz";
  resourceName = "server_bridge_dmz";
  name = "Server DMZ Bridge";
  mac = "82:13:8a:a0:4f:84";
  vlan = "dmz";
  addressKey = "server";
  dependsOn = [ "server_vlan_dmz" ];
}
