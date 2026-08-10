/*
  nixos-notify — report a host lifecycle event to Matrix by publishing to
  MQTT, which Home Assistant consumes via an mqtt trigger.

  Publishing to the broker (which stays up across HA restarts) is what fixes
  the old webhook race. Handles the boot / switch / scrub / gc events.

  Usage:
    nixos-notify <mosquitto_pub> <host> <port> <topic> <event> [arg] [arg2]
*/
{ writers }:
writers.writePython3Bin "nixos-notify" { } (builtins.readFile ./nixos-notify.py)
