# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-bridge-game";
  resourceName = "server_bridge_game";
  name = "Server Game Bridge";
  mac = "82:13:8a:a0:4f:70";
  vlan = "game";
  addressKey = "server";
  dependsOn = [ "server_vlan_game" ];
}
