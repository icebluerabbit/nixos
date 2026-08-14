# home network
{ den, ... }:
(import ../_mk-port-profile.nix { inherit den; }) {
  id = "storage";
  resourceName = "storage";
  displayName = "Storage Port";
  network = "storage";
  unifiId = "6a2e19a8efa3052c88b38ea3";
}
