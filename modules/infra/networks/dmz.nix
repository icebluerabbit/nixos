# modules/infra/networks/dmz.nix
{ den, lib, ... }:
(import ../_mk-network.nix { inherit den lib; }) {
  id = "dmz";
  resourceName = "dmz";
  displayName = "DMZ";
  vlanId = 100;
  subnet = "10.10.100.0";
  domain = "dmz.internal";
  staticAddresses = {
    gateway = "10.10.100.1";
    server = "10.10.100.2";
  };
}
