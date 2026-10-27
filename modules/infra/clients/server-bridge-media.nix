# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-bridge-media";
    resourceName = "server_bridge_media";
    name = "Server Media Bridge";
    mac = "82:13:8a:a0:4f:98";
    vlan = "media";
    kind = "virtual";
    dependsOn = [ "server_vlan_media" ];
  };
in
{
  den.aspects.infra._.clients._.server-bridge-media = {
    aspect-name = "infra.clients.server-bridge-media";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.media.staticAddresses.server;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.media.staticAddresses.server;
  };
}
