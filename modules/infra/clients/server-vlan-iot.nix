# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-vlan-iot";
  resourceName = "server_vlan_iot";
  name = "Server IoT";
  mac = "62:f5:a4:5b:20:1e";
}
