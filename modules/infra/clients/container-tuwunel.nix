# modules/infra/clients/container-tuwunel.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-tuwunel";
  resourceName = "container_tuwunel";
  name = "Tuwunel Container";
  mac = "62:f5:a4:5b:20:91";
  vlan = "services";
  addressKey = "tuwunel";
  ports.default = 6167;
  proxy.port = "default";
}
