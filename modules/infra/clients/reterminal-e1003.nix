# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "reterminal-e1003";
  resourceName = "reterminal_e1003";
  name = "reTerminal E1003";
  mac = "44:1b:f6:c0:04:54";
  vlan = "iot";
  addressKey = "reterminal-e1003";
}
