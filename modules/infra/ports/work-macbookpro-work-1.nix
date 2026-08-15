# home network
{ den, ... }:
let
  port = (import ../_mk-port-profile.nix null) {
    id = "work-macbookpro-work-1";
    resourceName = "work";
    displayName = "Work Port";
    network = "work";
    unifiId = "6a2e15faefa3052c88b38e11";
  };
in
{
  den.aspects.infra._.ports._.work-macbookpro-work-1 = {
    aspect-name = "infra.ports.work-macbookpro-work-1";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.work
    ];

    os = _: port.mkOs;
    terranix = _: port.mkTerranix;
  };
}
