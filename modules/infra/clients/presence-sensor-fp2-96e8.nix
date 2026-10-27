# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "presence-sensor-fp2-96e8";
    resourceName = "presence_sensor_fp2_96e8";
    name = "Presence Sensor FP2 96E8";
    mac = "54:ef:44:a1:96:e8";
    vlan = "iot";
    kind = "physical";
  };
in
{
  den.aspects.infra._.clients._.presence-sensor-fp2-96e8 = {
    aspect-name = "infra.clients.presence-sensor-fp2-96e8";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.iot
    ];

    os = _: client.mkOs "10.10.30.117";
    terranix = _: client.mkTerranix "10.10.30.117";
  };
}
