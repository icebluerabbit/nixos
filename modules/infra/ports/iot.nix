# home network
{ den, ... }:
(import ../_mk-port-profile.nix { inherit den; }) {
  id = "iot";
  resourceName = "iot";
  displayName = "IoT Port";
  network = "iot";
  unifiId = "6a2e15faefa3052c88b38e17";
}
