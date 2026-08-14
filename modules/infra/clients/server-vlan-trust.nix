# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-vlan-trust";
  resourceName = "server_vlan_trust";
  name = "Server Trust";
  mac = "62:f5:a4:5b:20:14";
}
