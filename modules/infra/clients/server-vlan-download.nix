# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-vlan-download";
    resourceName = "server_vlan_download";
    name = "Server Download";
    mac = "62:f5:a4:5b:20:82";
    vlan = "download";
    kind = "virtual";
  };
in
{
  den.aspects.infra._.clients._.server-vlan-download = {
    aspect-name = "infra.clients.server-vlan-download";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.download
    ];

    os = _: client.mkOs null;
    terranix = _: client.mkTerranix null;
  };
}
