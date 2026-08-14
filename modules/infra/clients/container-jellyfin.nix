# modules/infra/clients/container-jellyfin.nix
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-jellyfin";
  resourceName = "container_jellyfin";
  name = "Jellyfin Container";
  mac = "62:f5:a4:5b:20:7f";
  vlan = "media";
  addressKey = "jellyfin";
  ports.http = 8096;
  proxy.port = "http";
}
