# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-bridge-download";
  resourceName = "server_bridge_download";
  name = "Server Download Bridge";
  mac = "82:13:8a:a0:4f:a2";
  vlan = "download";
  addressKey = "server";
  dependsOn = [ "server_vlan_download" ];
}
