# home network
{ den, ... }:
(import ../_mk-usergroup.nix { inherit den; }) {
  id = "default";
  resourceName = "default";
  displayName = "Default";
  unifiId = "6a20c6afaa20ab3db2a1669e";
}
