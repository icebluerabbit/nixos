# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-vlan-apps";
    resourceName = "server_vlan_apps";
    name = "Server Apps";
    mac = "62:f5:a4:5b:20:6e";
    vlan = "apps";
    kind = "virtual";
  };
in
{
  den.aspects.infra._.clients._.server-vlan-apps = {
    aspect-name = "infra.clients.server-vlan-apps";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.apps
    ];

    os = _: client.mkOs null;
    terranix = _: client.mkTerranix null;
  };
}
