# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "container-qbittorrent";
  resourceName = "container_qbittorrent";
  name = "qBittorrent Container";
  mac = "62:f5:a4:5b:20:85";
  vlan = "download";
  addressKey = "qbittorrent";
  ports = {
    exporter = 8090;
    webui = 8080;
  };
  proxy.port = "webui";
}
