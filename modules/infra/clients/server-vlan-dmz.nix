# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-vlan-dmz";
    resourceName = "server_vlan_dmz";
    name = "Server DMZ";
    mac = "62:f5:a4:5b:20:64";
    vlan = "dmz";
    kind = "virtual";
  };
in
{
  den.aspects.infra._.clients._.server-vlan-dmz = {
    aspect-name = "infra.clients.server-vlan-dmz";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.dmz
    ];

    os = _: client.mkOs null;
    terranix = _: client.mkTerranix null;
  };
}
