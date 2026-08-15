# home network
{ den, ... }:
let
  port = (import ../_mk-port-profile.nix null) {
    id = "trust-desktop";
    resourceName = "trust";
    displayName = "Trust Port";
    network = "trust";
    unifiId = "6a3049b2efa3052c88b3c52f";
  };
in
{
  den.aspects.infra._.ports._.trust-desktop = {
    aspect-name = "infra.ports.trust-desktop";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.trust
    ];

    os = _: port.mkOs;
    terranix = _: port.mkTerranix;
  };
}
