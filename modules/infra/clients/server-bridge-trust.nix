# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-bridge-trust";
    resourceName = "server_bridge_trust";
    name = "Server Trust Bridge";
    mac = "82:13:8a:a0:4f:34";
    vlan = "trust";
    kind = "virtual";
    dependsOn = [ "server_vlan_trust" ];
  };
in
{
  den.aspects.infra._.clients._.server-bridge-trust = {
    aspect-name = "infra.clients.server-bridge-trust";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.trust
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.trust.staticAddresses.server;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.trust.staticAddresses.server;
  };
}
