# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "broadlink-remote";
    resourceName = "broadlink_remote";
    name = "BroadLink Remote";
    mac = "34:8e:89:2d:c1:19";
    vlan = "iot";
    kind = "physical";
  };
in
{
  den.aspects.infra._.clients._.broadlink-remote = {
    aspect-name = "infra.clients.broadlink-remote";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.iot
    ];

    os = _: client.mkOs "10.10.30.76";
    terranix = _: client.mkTerranix "10.10.30.76";
  };
}
