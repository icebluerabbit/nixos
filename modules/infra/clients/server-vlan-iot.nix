# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-vlan-iot";
    resourceName = "server_vlan_iot";
    name = "Server IoT";
    mac = "62:f5:a4:5b:20:1e";
    vlan = "iot";
    kind = "virtual";
  };
in
{
  den.aspects.infra._.clients._.server-vlan-iot = {
    aspect-name = "infra.clients.server-vlan-iot";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.iot
    ];

    os = _: client.mkOs null;
    terranix = _: client.mkTerranix null;
  };
}
