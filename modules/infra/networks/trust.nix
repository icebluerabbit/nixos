# modules/infra/networks/trust.nix
{ den, lib, ... }:
(import ../_mk-network.nix { inherit den lib; }) {
  id = "trust";
  resourceName = "trust";
  displayName = "TRUST";
  vlanId = 20;
  subnet = "10.10.20.0";
  domain = "trust.internal";
  staticAddresses = {
    gateway = "10.10.20.1";
    server = "10.10.20.2";
    desktop = "10.10.20.3";
  };
}
