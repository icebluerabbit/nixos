# modules/infra/clients/container-cinny.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-cinny";
  resourceName = "container_cinny";
  name = "Cinny Container";
  mac = "62:f5:a4:5b:20:74";
  vlan = "apps";
  addressKey = "cinny";
  ports.default = 80;
  proxy.port = "default";
}
