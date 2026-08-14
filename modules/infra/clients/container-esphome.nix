# modules/infra/clients/container-esphome.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-esphome";
  resourceName = "container_esphome";
  name = "ESPHome Container";
  mac = "62:f5:a4:5b:20:20";
  vlan = "iot";
  addressKey = "esphome";
}
