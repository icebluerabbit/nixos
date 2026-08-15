# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "macbookpro-work-2";
    resourceName = "macbookpro_work_2";
    name = "MacBookPro (work, wifi)";
    mac = "ee:17:4a:7c:e1:18";
    vlan = "work";
    kind = "physical";
  };
in
{
  den.aspects.infra._.clients._.macbookpro-work-2 = {
    aspect-name = "infra.clients.macbookpro-work-2";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.work
    ];

    os = _: client.mkOs "10.10.70.42";
    terranix = _: client.mkTerranix "10.10.70.42";
  };
}
