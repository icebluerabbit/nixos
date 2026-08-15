# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-bridge-guest";
    resourceName = "server_bridge_guest";
    name = "Server Guest Bridge";
    mac = "82:13:8a:a0:4f:48";
    vlan = "guest";
    kind = "virtual";
    dependsOn = [ "server_vlan_guest" ];
  };
in
{
  den.aspects.infra._.clients._.server-bridge-guest = {
    aspect-name = "infra.clients.server-bridge-guest";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.guest
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.guest.staticAddresses.server;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.guest.staticAddresses.server;
  };
}
