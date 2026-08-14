# modules/infra/clients/container-mosquitto.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-mosquitto";
  resourceName = "container_mosquitto";
  name = "Mosquitto Container";
  mac = "62:f5:a4:5b:20:22";
  vlan = "iot";
  addressKey = "mosquitto";
  ports.default = 1883;
}
