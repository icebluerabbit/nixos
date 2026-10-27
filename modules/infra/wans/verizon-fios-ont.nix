# home network
{ den, lib, ... }:
let
  wan = (import ../_mk-wan.nix { inherit lib; }) {
    id = "verizon-fios-ont";
    resourceName = "verizon_fios_ont";
    displayName = "Verizon Fios ONT";
    unifiId = "6a20c6afaa20ab3db2a1669b";
    uploadKbps = 2543000;
    downloadKbps = 2365000;
  };
in
{
  den.aspects.infra._.wans._.verizon-fios-ont = {
    aspect-name = "infra.wans.verizon-fios-ont";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: wan.mkOs;
    terranix = _: wan.mkTerranix;
  };
}
