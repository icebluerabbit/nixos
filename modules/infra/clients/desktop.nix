# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "desktop";
    resourceName = "desktop";
    name = "Desktop";
    mac = "b0:82:e2:32:f9:94";
    vlan = "trust";
    kind = "physical";
    portProfile = "trust-desktop";
  };
in
{
  den.aspects.infra._.clients._.desktop = {
    aspect-name = "infra.clients.desktop";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.trust
      den.aspects.infra._.ports._.trust-desktop
    ];

    os = { config, ... }: client.mkOs config.infra.vlans.trust.staticAddresses.desktop;
    terranix = { config, ... }: client.mkTerranix config.infra.vlans.trust.staticAddresses.desktop;
  };
}
