# home network
{ den, ... }:
(import ../_mk-port-profile.nix { inherit den; }) {
  id = "mgmt";
  resourceName = "mgmt";
  displayName = "MGMT Port";
  network = "mgmt";
  unifiId = "6a2e2450efa3052c88b38fcb";
}
