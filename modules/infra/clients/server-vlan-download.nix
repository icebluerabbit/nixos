# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-vlan-download";
  resourceName = "server_vlan_download";
  name = "Server Download";
  mac = "62:f5:a4:5b:20:82";
}
