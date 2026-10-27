# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-bridge-services";
    resourceName = "server_bridge_services";
    name = "Server Services Bridge";
    mac = "82:13:8a:a0:4f:ac";
    vlan = "services";
    kind = "virtual";
    dependsOn = [ "server_vlan_services" ];
  };
in
{
  den.aspects.infra._.clients._.server-bridge-services = {
    aspect-name = "infra.clients.server-bridge-services";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.services
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.services.staticAddresses.server;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.services.staticAddresses.server;
  };
}
