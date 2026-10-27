# home network
{ den, ... }:
let
  port = (import ../_mk-port-profile.nix null) {
    id = "mgmt-server";
    resourceName = "mgmt";
    displayName = "MGMT Port";
    network = "mgmt";
    unifiId = "6a2e2450efa3052c88b38fcb";
  };
in
{
  den.aspects.infra._.ports._.mgmt-server = {
    aspect-name = "infra.ports.mgmt-server";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.mgmt
    ];

    os = _: port.mkOs;
    terranix = _: port.mkTerranix;
  };
}
