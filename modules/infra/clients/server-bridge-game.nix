# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-bridge-game";
    resourceName = "server_bridge_game";
    name = "Server Game Bridge";
    mac = "82:13:8a:a0:4f:70";
    vlan = "game";
    kind = "virtual";
    dependsOn = [ "server_vlan_game" ];
  };
in
{
  den.aspects.infra._.clients._.server-bridge-game = {
    aspect-name = "infra.clients.server-bridge-game";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.game
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.game.staticAddresses.server;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.game.staticAddresses.server;
  };
}
