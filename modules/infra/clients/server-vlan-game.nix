# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-vlan-game";
  resourceName = "server_vlan_game";
  name = "Server Game";
  mac = "62:f5:a4:5b:20:50";
}
