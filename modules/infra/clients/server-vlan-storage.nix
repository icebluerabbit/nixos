# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-vlan-storage";
  resourceName = "server_vlan_storage";
  name = "Server Storage";
  mac = "62:f5:a4:5b:20:32";
}
