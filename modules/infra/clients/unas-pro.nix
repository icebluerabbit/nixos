# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "unas-pro";
  resourceName = "unas_pro";
  name = "UNAS-Pro";
  mac = "6c:63:f8:10:41:a2";
  vlan = "storage";
  addressKey = "unas-pro";
}
