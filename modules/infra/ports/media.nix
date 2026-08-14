# home network
{ den, ... }:
(import ../_mk-port-profile.nix { inherit den; }) {
  id = "media";
  resourceName = "media";
  displayName = "Media Port";
  network = "media";
  unifiId = "6a3049b2efa3052c88b3c52d";
}
