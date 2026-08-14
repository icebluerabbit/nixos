# modules/infra/networks/storage.nix
{ den, lib, ... }:
(import ../_mk-network.nix { inherit den lib; }) {
  id = "storage";
  resourceName = "storage";
  displayName = "STORAGE";
  vlanId = 50;
  subnet = "10.10.50.0";
  domain = "storage.internal";
  staticAddresses = {
    gateway = "10.10.50.1";
    server = "10.10.50.2";
    unas-pro = "10.10.50.222";
  };
}
