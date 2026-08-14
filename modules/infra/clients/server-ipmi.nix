# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "server-ipmi";
  resourceName = "server_ipmi";
  name = "Server IPMI";
  mac = "9c:6b:00:d2:25:5f";
  vlan = "mgmt";
  addressKey = "server-ipmi";
}
