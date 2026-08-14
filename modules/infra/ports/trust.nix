# home network
{ den, ... }:
(import ../_mk-port-profile.nix { inherit den; }) {
  id = "trust";
  resourceName = "trust";
  displayName = "Trust Port";
  network = "trust";
  unifiId = "6a3049b2efa3052c88b3c52f";
}
