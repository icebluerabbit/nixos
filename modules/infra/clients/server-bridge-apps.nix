# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-bridge-apps";
    resourceName = "server_bridge_apps";
    name = "Server Apps Bridge";
    mac = "82:13:8a:a0:4f:8e";
    vlan = "apps";
    kind = "virtual";
    dependsOn = [ "server_vlan_apps" ];
  };
in
{
  den.aspects.infra._.clients._.server-bridge-apps = {
    aspect-name = "infra.clients.server-bridge-apps";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.apps
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.apps.staticAddresses.server;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.apps.staticAddresses.server;
  };
}
