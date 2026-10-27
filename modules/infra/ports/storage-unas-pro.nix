# home network
{ den, ... }:
let
  port = (import ../_mk-port-profile.nix null) {
    id = "storage-unas-pro";
    resourceName = "storage";
    displayName = "Storage Port";
    network = "storage";
    unifiId = "6a2e19a8efa3052c88b38ea3";
  };
in
{
  den.aspects.infra._.ports._.storage-unas-pro = {
    aspect-name = "infra.ports.storage-unas-pro";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.storage
    ];

    os = _: port.mkOs;
    terranix = _: port.mkTerranix;
  };
}
