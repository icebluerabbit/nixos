# modules/infra/networks/work.nix
{ den, lib, ... }:
(import ../_mk-network.nix { inherit den lib; }) {
  id = "work";
  resourceName = "work";
  displayName = "WORK";
  vlanId = 70;
  subnet = "10.10.70.0";
  domain = "work.internal";
  isolated = true; # Isolated from other local VLANs
  dnsServers = [
    "1.1.1.1"
    "8.8.8.8"
  ];
  staticAddresses = {
    gateway = "10.10.70.1";
    server = "10.10.70.2";
  };
}
