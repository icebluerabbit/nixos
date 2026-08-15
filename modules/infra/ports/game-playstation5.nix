# home network
{ den, ... }:
let
  port = (import ../_mk-port-profile.nix null) {
    id = "game-playstation5";
    resourceName = "game";
    displayName = "Game Port";
    network = "game";
    unifiId = "6a2e15faefa3052c88b38e13";
  };
in
{
  den.aspects.infra._.ports._.game-playstation5 = {
    aspect-name = "infra.ports.game-playstation5";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.game
    ];

    os = _: port.mkOs;
    terranix = _: port.mkTerranix;
  };
}
