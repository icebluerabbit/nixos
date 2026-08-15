# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-vlan-media";
    resourceName = "server_vlan_media";
    name = "Server Media";
    mac = "62:f5:a4:5b:20:78";
    vlan = "media";
    kind = "virtual";
  };
in
{
  den.aspects.infra._.clients._.server-vlan-media = {
    aspect-name = "infra.clients.server-vlan-media";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.media
    ];

    os = _: client.mkOs null;
    terranix = _: client.mkTerranix null;
  };
}
