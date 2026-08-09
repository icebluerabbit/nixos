{ config, ... }:
{
  name = "qBittorrent";
  implementation = "QBittorrent";
  enable = true;
  protocol = "torrent";
  remove_failed_downloads = true;
  # Never remove completed torrents — seed forever (removal is qbt/manual only).
  remove_completed_downloads = false;
  host = config.infra.clients.container-qbittorrent.ip;
  port = config.infra.clients.container-qbittorrent.ports.webui;
  tv_category = "tv";
}
