# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server";
  resourceName = "server";
  name = "Server";
  mac = "9c:6b:00:d2:18:a6";
  vlan = "mgmt";
  addressKey = "server";
}
