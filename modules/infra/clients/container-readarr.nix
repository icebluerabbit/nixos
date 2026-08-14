# modules/infra/clients/container-readarr.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-readarr";
  resourceName = "container_readarr";
  name = "Readarr Container";
  mac = "62:f5:a4:5b:20:7b";
  vlan = "media";
  addressKey = "readarr";
  ports.default = 8787;
  proxy.port = "default";
}
