# home network
{ den, ... }:
(import ../_mk-port-profile.nix { inherit den; }) {
  id = "dmz";
  resourceName = "dmz";
  displayName = "DMZ Port";
  network = "dmz";
  unifiId = "6a3049b2efa3052c88b3c531";
}
