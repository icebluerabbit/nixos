# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-ipmi";
    resourceName = "server_ipmi";
    name = "Server IPMI";
    mac = "9c:6b:00:d2:25:5f";
    vlan = "mgmt";
    kind = "physical";
    portProfile = "mgmt-server-ipmi";
  };
in
{
  den.aspects.infra._.clients._.server-ipmi = {
    aspect-name = "infra.clients.server-ipmi";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.mgmt
      den.aspects.infra._.ports._.mgmt-server-ipmi
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.mgmt.staticAddresses.server-ipmi;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.mgmt.staticAddresses.server-ipmi;
  };
}
