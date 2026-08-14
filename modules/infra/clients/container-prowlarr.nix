# modules/infra/clients/container-prowlarr.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-prowlarr";
  resourceName = "container_prowlarr";
  name = "Prowlarr Container";
  mac = "62:f5:a4:5b:20:7d";
  vlan = "media";
  addressKey = "prowlarr";
  ports = {
    default = 9696;
    exporter = 9707;
  };
  proxy.port = "default";
}
