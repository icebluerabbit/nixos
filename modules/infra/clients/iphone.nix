# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "iphone";
    resourceName = "iphone";
    name = "iPhone";
    mac = "d6:c8:14:8c:ab:3e";
    vlan = "trust";
    kind = "physical";
  };
in
{
  den.aspects.infra._.clients._.iphone = {
    aspect-name = "infra.clients.iphone";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.trust
    ];

    os = _: client.mkOs "10.10.20.60";
    terranix = _: client.mkTerranix "10.10.20.60";
  };
}
