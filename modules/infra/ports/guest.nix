# home network
{ den, ... }:
(import ../_mk-port-profile.nix { inherit den; }) {
  id = "guest";
  resourceName = "guest";
  displayName = "Guest Port";
  network = "guest";
  unifiId = "6a3049b2efa3052c88b3c52a";
}
