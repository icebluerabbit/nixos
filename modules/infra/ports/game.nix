# home network
{ den, ... }:
(import ../_mk-port-profile.nix { inherit den; }) {
  id = "game";
  resourceName = "game";
  displayName = "Game Port";
  network = "game";
  unifiId = "6a2e15faefa3052c88b38e13";
}
