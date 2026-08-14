# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-bridge-apps";
  resourceName = "server_bridge_apps";
  name = "Server Apps Bridge";
  mac = "82:13:8a:a0:4f:8e";
  vlan = "apps";
  addressKey = "server";
  dependsOn = [ "server_vlan_apps" ];
}
