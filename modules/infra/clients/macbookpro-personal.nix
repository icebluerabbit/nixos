# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "macbookpro-personal";
    resourceName = "macbookpro_personal";
    name = "MacBookPro (personal)";
    mac = "66:87:b9:92:9c:77";
    vlan = "trust";
    kind = "physical";
  };
in
{
  den.aspects.infra._.clients._.macbookpro-personal = {
    aspect-name = "infra.clients.macbookpro-personal";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.trust
    ];

    os = _: client.mkOs "10.10.20.86";
    terranix = _: client.mkTerranix "10.10.20.86";
  };
}
