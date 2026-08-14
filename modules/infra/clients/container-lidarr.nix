# modules/infra/clients/container-lidarr.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-lidarr";
  resourceName = "container_lidarr";
  name = "Lidarr Container";
  mac = "62:f5:a4:5b:20:7a";
  vlan = "media";
  addressKey = "lidarr";
  ports = {
    default = 8686;
    exporter = 9707;
  };
  proxy.port = "default";
}
