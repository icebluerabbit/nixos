# home network
{ den, ... }:
(import ../_mk-port-profile.nix { inherit den; }) {
  id = "work";
  resourceName = "work";
  displayName = "Work Port";
  network = "work";
  unifiId = "6a2e15faefa3052c88b38e11";
}
