# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "ipad";
    resourceName = "ipad";
    name = "iPad";
    mac = "4e:37:93:b9:e0:6a";
    vlan = "trust";
    kind = "physical";
  };
in
{
  den.aspects.infra._.clients._.ipad = {
    aspect-name = "infra.clients.ipad";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.trust
    ];

    os = _: client.mkOs "10.10.20.50";
    terranix = _: client.mkTerranix "10.10.20.50";
  };
}
