# modules/infra/clients/container-youtarr.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-youtarr";
  resourceName = "container_youtarr";
  name = "Youtarr Container";
  mac = "62:f5:a4:5b:20:7e";
  vlan = "media";
  addressKey = "youtarr";
  ports.http = 3011;
  proxy.port = "http";
}
