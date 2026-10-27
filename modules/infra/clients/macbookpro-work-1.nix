# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "macbookpro-work-1";
    resourceName = "macbookpro_work_1";
    name = "MacBookPro (work 1)";
    mac = "a0:ce:c8:a0:f1:27";
    vlan = "work";
    kind = "physical";
    portProfile = "work-macbookpro-work-1";
  };
in
{
  den.aspects.infra._.clients._.macbookpro-work-1 = {
    aspect-name = "infra.clients.macbookpro-work-1";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.work
      den.aspects.infra._.ports._.work-macbookpro-work-1
    ];

    os = _: client.mkOs "10.10.70.243";
    terranix = _: client.mkTerranix "10.10.70.243";
  };
}
