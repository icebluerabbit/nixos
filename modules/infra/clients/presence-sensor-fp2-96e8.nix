# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "presence-sensor-fp2-96e8";
  resourceName = "presence_sensor_fp2_96e8";
  name = "Presence Sensor FP2 96E8";
  mac = "54:ef:44:a1:96:e8";
  ip = "10.10.30.117";
}
