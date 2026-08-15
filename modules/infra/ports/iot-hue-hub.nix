# home network
{ den, ... }:
let
  port = (import ../_mk-port-profile.nix null) {
    id = "iot-hue-hub";
    resourceName = "iot";
    displayName = "IoT Port";
    network = "iot";
    unifiId = "6a2e15faefa3052c88b38e17";
  };
in
{
  den.aspects.infra._.ports._.iot-hue-hub = {
    aspect-name = "infra.ports.iot-hue-hub";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.iot
    ];

    os = _: port.mkOs;
    terranix = _: port.mkTerranix;
  };
}
