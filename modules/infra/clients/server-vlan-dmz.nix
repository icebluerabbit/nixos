# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-vlan-dmz";
  resourceName = "server_vlan_dmz";
  name = "Server DMZ";
  mac = "62:f5:a4:5b:20:64";
}
