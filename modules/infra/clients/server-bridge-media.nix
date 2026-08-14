# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-bridge-media";
  resourceName = "server_bridge_media";
  name = "Server Media Bridge";
  mac = "82:13:8a:a0:4f:98";
  vlan = "media";
  addressKey = "server";
  dependsOn = [ "server_vlan_media" ];
}
