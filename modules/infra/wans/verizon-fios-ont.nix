# home network
{ den, ... }:
(import ../_mk-wan.nix { inherit den; }) {
  id = "verizon-fios-ont";
  resourceName = "verizon_fios_ont";
  displayName = "Verizon Fios ONT";
  unifiId = "6a20c6afaa20ab3db2a1669b";
  uploadKbps = 2543000;
  downloadKbps = 2365000;
}
