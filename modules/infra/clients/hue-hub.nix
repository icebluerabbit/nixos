# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "hue-hub";
    resourceName = "hue_hub";
    name = "Hue Hub";
    mac = "00:17:88:79:81:e0";
    vlan = "iot";
    kind = "physical";
    portProfile = "iot-hue-hub";
  };
in
{
  den.aspects.infra._.clients._.hue-hub = {
    aspect-name = "infra.clients.hue-hub";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.iot
      den.aspects.infra._.ports._.iot-hue-hub
    ];

    os = _: client.mkOs "10.10.30.33";
    terranix = _: client.mkTerranix "10.10.30.33";
  };
}
