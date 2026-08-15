# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-bridge-work";
    resourceName = "server_bridge_work";
    name = "Server Work Bridge";
    mac = "82:13:8a:a0:4f:66";
    vlan = "work";
    kind = "virtual";
    dependsOn = [ "server_vlan_work" ];
  };
in
{
  den.aspects.infra._.clients._.server-bridge-work = {
    aspect-name = "infra.clients.server-bridge-work";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.work
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.work.staticAddresses.server;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.work.staticAddresses.server;
  };
}
