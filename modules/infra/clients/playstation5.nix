# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "playstation5";
    resourceName = "playstation5";
    name = "Sony PlayStation 5";
    mac = "b4:0a:d8:2b:e5:68";
    vlan = "game";
    kind = "physical";
    portProfile = "game-playstation5";
  };
in
{
  den.aspects.infra._.clients._.playstation5 = {
    aspect-name = "infra.clients.playstation5";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.game
      den.aspects.infra._.ports._.game-playstation5
    ];

    os = _: client.mkOs "10.10.80.45";
    terranix = _: client.mkTerranix "10.10.80.45";
  };
}
