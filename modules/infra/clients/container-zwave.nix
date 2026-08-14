# modules/infra/clients/container-zwave.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-zwave";
  resourceName = "container_zwave";
  name = "Z-Wave Container";
  mac = "62:f5:a4:5b:20:21";
  vlan = "iot";
  addressKey = "zwave";
  ports = {
    ui = 8091;
    ws = 3000;
  };
  proxy.port = "ui";
}
