# modules/infra/clients/container-radarr.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-radarr";
  resourceName = "container_radarr";
  name = "Radarr Container";
  mac = "62:f5:a4:5b:20:79";
  vlan = "media";
  addressKey = "radarr";
  ports = {
    default = 7878;
    exporter = 9707;
  };
  proxy.port = "default";
}
