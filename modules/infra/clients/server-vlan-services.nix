# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-vlan-services";
  resourceName = "server_vlan_services";
  name = "Server Services";
  mac = "62:f5:a4:5b:20:8c";
}
