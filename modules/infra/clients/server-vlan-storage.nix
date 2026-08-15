# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-vlan-storage";
    resourceName = "server_vlan_storage";
    name = "Server Storage";
    mac = "62:f5:a4:5b:20:32";
    vlan = "storage";
    kind = "virtual";
  };
in
{
  den.aspects.infra._.clients._.server-vlan-storage = {
    aspect-name = "infra.clients.server-vlan-storage";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.storage
    ];

    os = _: client.mkOs null;
    terranix = _: client.mkTerranix null;
  };
}
