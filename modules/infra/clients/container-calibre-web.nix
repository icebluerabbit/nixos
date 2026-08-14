# modules/infra/clients/container-calibre-web.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-calibre-web";
  resourceName = "container_calibre_web";
  name = "Calibre-Web Container";
  mac = "62:f5:a4:5b:20:86";
  vlan = "media";
  addressKey = "calibre-web";
  ports.default = 8083;
  proxy.port = "default";
}
