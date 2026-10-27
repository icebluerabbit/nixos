# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-vlan-guest";
    resourceName = "server_vlan_guest";
    name = "Server Guest";
    mac = "62:f5:a4:5b:20:28";
    vlan = "guest";
    kind = "virtual";
  };
in
{
  den.aspects.infra._.clients._.server-vlan-guest = {
    aspect-name = "infra.clients.server-vlan-guest";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.guest
    ];

    os = _: client.mkOs null;
    terranix = _: client.mkTerranix null;
  };
}
