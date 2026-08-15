# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-bridge-dmz";
    resourceName = "server_bridge_dmz";
    name = "Server DMZ Bridge";
    mac = "82:13:8a:a0:4f:84";
    vlan = "dmz";
    kind = "virtual";
    dependsOn = [ "server_vlan_dmz" ];
  };
in
{
  den.aspects.infra._.clients._.server-bridge-dmz = {
    aspect-name = "infra.clients.server-bridge-dmz";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.dmz
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.dmz.staticAddresses.server;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.dmz.staticAddresses.server;
  };
}
