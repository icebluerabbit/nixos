# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "presence-sensor-fp2-db2e";
  resourceName = "presence_sensor_fp2_db2e";
  name = "Presence Sensor FP2 DB2E";
  mac = "54:ef:44:a3:db:2e";
  ip = "10.10.30.165";
}
