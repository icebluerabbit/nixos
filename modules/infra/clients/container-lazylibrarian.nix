# modules/infra/clients/container-lazylibrarian.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-lazylibrarian";
  resourceName = "container_lazylibrarian";
  name = "LazyLibrarian Container";
  mac = "62:f5:a4:5b:20:80";
  vlan = "media";
  addressKey = "lazylibrarian";
  ports.default = 5299;
  proxy.port = "default";
}
