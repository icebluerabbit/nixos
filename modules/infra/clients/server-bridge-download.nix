# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-bridge-download";
    resourceName = "server_bridge_download";
    name = "Server Download Bridge";
    mac = "82:13:8a:a0:4f:a2";
    vlan = "download";
    kind = "virtual";
    dependsOn = [ "server_vlan_download" ];
  };
in
{
  den.aspects.infra._.clients._.server-bridge-download = {
    aspect-name = "infra.clients.server-bridge-download";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.download
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.download.staticAddresses.server;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.download.staticAddresses.server;
  };
}
