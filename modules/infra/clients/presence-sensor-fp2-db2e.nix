# home network
{ den, lib, ... }:
let
  client = (import ../_mk-client.nix { inherit lib; }) {
    id = "presence-sensor-fp2-db2e";
    resourceName = "presence_sensor_fp2_db2e";
    name = "Presence Sensor FP2 DB2E";
    mac = "54:ef:44:a3:db:2e";
    vlan = "iot";
    kind = "physical";
  };
in
{
  den.aspects.infra._.clients._.presence-sensor-fp2-db2e = {
    aspect-name = "infra.clients.presence-sensor-fp2-db2e";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.iot
    ];

    os = _: client.mkOs "10.10.30.165";
    terranix = _: client.mkTerranix "10.10.30.165";
  };
}
