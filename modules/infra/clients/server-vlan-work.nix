# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-vlan-work";
    resourceName = "server_vlan_work";
    name = "Server Work";
    mac = "62:f5:a4:5b:20:46";
    vlan = "work";
    kind = "virtual";
  };
in
{
  den.aspects.infra._.clients._.server-vlan-work = {
    aspect-name = "infra.clients.server-vlan-work";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.work
    ];

    os = _: client.mkOs null;
    terranix = _: client.mkTerranix null;
  };
}
