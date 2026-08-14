# home network
{ den, ... }:
(import ../_mk-port-profile.nix { inherit den; }) {
  id = "download";
  resourceName = "download";
  displayName = "Download Port";
  network = "download";
  unifiId = "6a3049b2efa3052c88b3c52e";
}
