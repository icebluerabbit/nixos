# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "reterminal-e1003";
    resourceName = "reterminal_e1003";
    name = "reTerminal E1003";
    mac = "44:1b:f6:c0:04:54";
    vlan = "iot";
    kind = "physical";
  };
in
{
  den.aspects.infra._.clients._.reterminal-e1003 = {
    aspect-name = "infra.clients.reterminal-e1003";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.iot
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.iot.staticAddresses.reterminal-e1003;
    terranix =
      { config, ... }: client.mkTerranix config.infra.vlans.iot.staticAddresses.reterminal-e1003;
  };
}
