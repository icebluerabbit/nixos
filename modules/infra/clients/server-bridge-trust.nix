# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-bridge-trust";
  resourceName = "server_bridge_trust";
  name = "Server Trust Bridge";
  mac = "82:13:8a:a0:4f:34";
  vlan = "trust";
  addressKey = "server";
  dependsOn = [ "server_vlan_trust" ];
}
