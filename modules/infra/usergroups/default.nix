# home network
{ den, ... }:
let
  usergroup = (import ../_mk-usergroup.nix null) {
    resourceName = "default";
    displayName = "Default";
    unifiId = "6a20c6afaa20ab3db2a1669e";
  };
in
{
  den.aspects.infra._.usergroups._.default = {
    aspect-name = "infra.usergroups.default";
    includes = [
      den.aspects.infra._.providers._.unifi
    ];

    terranix = _: usergroup.mkTerranix;
  };
}
