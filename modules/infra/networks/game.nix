# modules/infra/networks/game.nix
{ den, lib, ... }:
(import ../_mk-network.nix { inherit den lib; }) {
  id = "game";
  resourceName = "game";
  displayName = "GAME";
  vlanId = 80;
  subnet = "10.10.80.0";
  domain = "game.internal";
  isolated = true; # Isolated from other local VLANs
  staticAddresses = {
    gateway = "10.10.80.1";
    server = "10.10.80.2";
  };
}
