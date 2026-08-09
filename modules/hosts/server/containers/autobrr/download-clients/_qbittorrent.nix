# Direct qBittorrent client, used only by the seasonpackarr filter. Every other
# TV/movie filter hands off to an *arr, which owns quality decisions and grabs
# through its own client — this one exists because seasonpackarr v0.16.0 only
# hardlinks the episodes into the pack folder and leaves adding the torrent to
# autobrr.
#
# No credentials: autobrr's media-VLAN IP is in qBittorrent's
# authSubnetWhitelist, so the WebUI bypasses auth for it.
{ config, ... }:
{
  name = "qBittorrent";
  client_type = "QBITTORRENT";
  enabled = true;
  host = "http://${config.infra.clients.container-qbittorrent.ip}";
  port = config.infra.clients.container-qbittorrent.ports.webui;
}
