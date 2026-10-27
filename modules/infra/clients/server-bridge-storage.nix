# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-bridge-storage";
    resourceName = "server_bridge_storage";
    name = "Server Storage Bridge";
    mac = "82:13:8a:a0:4f:52";
    vlan = "storage";
    kind = "virtual";
    dependsOn = [ "server_vlan_storage" ];
  };
in
{
  den.aspects.infra._.clients._.server-bridge-storage = {
    aspect-name = "infra.clients.server-bridge-storage";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.storage
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.storage.staticAddresses.server;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.storage.staticAddresses.server;
  };
}
