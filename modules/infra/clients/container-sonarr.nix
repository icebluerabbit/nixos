# modules/infra/clients/container-sonarr.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-sonarr";
  resourceName = "container_sonarr";
  name = "Sonarr Container";
  mac = "62:f5:a4:5b:20:7c";
  vlan = "media";
  addressKey = "sonarr";
  ports = {
    default = 8989;
    exporter = 9707;
  };
  proxy.port = "default";
}
