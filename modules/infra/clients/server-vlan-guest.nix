# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-vlan-guest";
  resourceName = "server_vlan_guest";
  name = "Server Guest";
  mac = "62:f5:a4:5b:20:28";
}
