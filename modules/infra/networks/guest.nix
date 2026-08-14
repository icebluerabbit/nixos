# modules/infra/networks/guest.nix
{ den, lib, ... }:
(import ../_mk-network.nix { inherit den lib; }) {
  id = "guest";
  resourceName = "guest";
  displayName = "GUEST";
  vlanId = 40;
  subnet = "10.10.40.0";
  domain = "guest.internal";
  isolated = true; # Isolated from other local VLANs
  staticAddresses = {
    gateway = "10.10.40.1";
    server = "10.10.40.2";
  };
}
