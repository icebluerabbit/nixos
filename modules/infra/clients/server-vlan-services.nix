# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "server-vlan-services";
    resourceName = "server_vlan_services";
    name = "Server Services";
    mac = "62:f5:a4:5b:20:8c";
    vlan = "services";
    kind = "virtual";
  };
in
{
  den.aspects.infra._.clients._.server-vlan-services = {
    aspect-name = "infra.clients.server-vlan-services";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.services
    ];

    os = _: client.mkOs null;
    terranix = _: client.mkTerranix null;
  };
}
