# modules/infra/networks/mgmt.nix
{ den, lib, ... }:
(import ../_mk-network.nix { inherit den lib; }) {
  id = "mgmt";
  resourceName = "mgmt";
  displayName = "MGMT";
  vlanId = 10;
  subnet = "10.10.10.0";
  domain = "mgmt.internal";
  staticAddresses = {
    gateway = "10.10.10.1";
    server = "10.10.10.2";
    server-ipmi = "10.10.10.3";
  };
}
