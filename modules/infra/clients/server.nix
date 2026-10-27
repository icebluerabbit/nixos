# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server";
    resourceName = "server";
    name = "Server";
    mac = "9c:6b:00:d2:18:a6";
    vlan = "mgmt";
    kind = "physical";
    portProfile = "mgmt-server";
  };
in
{
  den.aspects.infra._.clients._.server = {
    aspect-name = "infra.clients.server";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.mgmt
      den.aspects.infra._.ports._.mgmt-server
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.mgmt.staticAddresses.server;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.mgmt.staticAddresses.server;
  };
}
