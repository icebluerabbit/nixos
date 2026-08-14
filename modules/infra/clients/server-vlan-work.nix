# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-vlan-work";
  resourceName = "server_vlan_work";
  name = "Server Work";
  mac = "62:f5:a4:5b:20:46";
}
