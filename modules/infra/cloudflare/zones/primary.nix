{ den, ... }:
(import ../../_mk-zone.nix { inherit den; }) {
  id = "primary";
  resourceName = "primary";
  nameVariable = "cf_zone_name";
  accountVariable = "cf_account_id";
  importVariable = "cf_zone_id";
}
