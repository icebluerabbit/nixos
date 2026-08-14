# modules/infra/clients/container-kiwix.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-kiwix";
  resourceName = "container_kiwix";
  name = "Kiwix Container";
  mac = "62:f5:a4:5b:20:72";
  vlan = "apps";
  addressKey = "kiwix";
  ports.http = 8080;
  proxy.port = "http";
}
