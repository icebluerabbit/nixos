# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-bridge-services";
  resourceName = "server_bridge_services";
  name = "Server Services Bridge";
  mac = "82:13:8a:a0:4f:ac";
  vlan = "services";
  addressKey = "server";
  dependsOn = [ "server_vlan_services" ];
}
