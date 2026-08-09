{ config, ... }:
{
  name = "qBittorrent";
  implementation = "QBittorrent";
  enable = true;
  protocol = "torrent";
  host = config.infra.clients.container-qbittorrent.ip;
  port = config.infra.clients.container-qbittorrent.ports.webui;
}
