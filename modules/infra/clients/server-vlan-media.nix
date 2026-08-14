# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-vlan-media";
  resourceName = "server_vlan_media";
  name = "Server Media";
  mac = "62:f5:a4:5b:20:78";
}
