# modules/infra/networks/iot.nix
{ den, lib, ... }:
(import ../_mk-network.nix { inherit den lib; }) {
  id = "iot";
  resourceName = "iot";
  displayName = "IOT";
  vlanId = 30;
  subnet = "10.10.30.0";
  domain = "iot.internal";
  staticAddresses = {
    gateway = "10.10.30.1";
    server = "10.10.30.2";
    home-assistant = "10.10.30.3";
    reterminal-e1003 = "10.10.30.4";
    esphome = "10.10.30.5";
    zwave = "10.10.30.6";
    mosquitto = "10.10.30.7";
    zigbee2mqtt = "10.10.30.8";
  };
}
