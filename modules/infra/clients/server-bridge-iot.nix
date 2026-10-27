# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-bridge-iot";
    resourceName = "server_bridge_iot";
    name = "Server IoT Bridge";
    mac = "82:13:8a:a0:4f:3e";
    vlan = "iot";
    kind = "virtual";
    dependsOn = [ "server_vlan_iot" ];
  };
in
{
  den.aspects.infra._.clients._.server-bridge-iot = {
    aspect-name = "infra.clients.server-bridge-iot";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.iot
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.iot.staticAddresses.server;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.iot.staticAddresses.server;
  };
}
