# modules/infra/clients/container-home-assistant.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-home-assistant";
  resourceName = "container_home_assistant";
  name = "Home Assistant Container";
  mac = "62:f5:a4:5b:20:1f";
  vlan = "iot";
  addressKey = "home-assistant";
  ports.http = 8123;
  proxy.port = "http";
}
