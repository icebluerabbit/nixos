# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-vlan-trust";
    resourceName = "server_vlan_trust";
    name = "Server Trust";
    mac = "62:f5:a4:5b:20:14";
    vlan = "trust";
    kind = "virtual";
  };
in
{
  den.aspects.infra._.clients._.server-vlan-trust = {
    aspect-name = "infra.clients.server-vlan-trust";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.trust
    ];

    os = _: client.mkOs null;
    terranix = _: client.mkTerranix null;
  };
}
