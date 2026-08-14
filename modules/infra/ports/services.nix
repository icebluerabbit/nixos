# home network
{ den, ... }:
(import ../_mk-port-profile.nix { inherit den; }) {
  id = "services";
  resourceName = "services";
  displayName = "Services Port";
  network = "services";
  unifiId = "6a3049b2efa3052c88b3c52c";
}
