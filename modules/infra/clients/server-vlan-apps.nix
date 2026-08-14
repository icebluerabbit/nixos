# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-vlan-apps";
  resourceName = "server_vlan_apps";
  name = "Server Apps";
  mac = "62:f5:a4:5b:20:6e";
}
