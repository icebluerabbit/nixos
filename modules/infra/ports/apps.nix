# home network
{ den, ... }:
(import ../_mk-port-profile.nix { inherit den; }) {
  id = "apps";
  resourceName = "apps";
  displayName = "Apps Port";
  network = "apps";
  unifiId = "6a3049b2efa3052c88b3c541";
}
