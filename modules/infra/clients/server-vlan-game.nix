# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-vlan-game";
    resourceName = "server_vlan_game";
    name = "Server Game";
    mac = "62:f5:a4:5b:20:50";
    vlan = "game";
    kind = "virtual";
  };
in
{
  den.aspects.infra._.clients._.server-vlan-game = {
    aspect-name = "infra.clients.server-vlan-game";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.game
    ];

    os = _: client.mkOs null;
    terranix = _: client.mkTerranix null;
  };
}
