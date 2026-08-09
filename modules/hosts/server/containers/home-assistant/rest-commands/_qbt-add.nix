# Manually push an autobrr-rejected release into qbittorrent on a 👍 approval.
# qbt's WebUI auth-subnet whitelist includes HA's /32 (HA lives on the iot VLAN),
# so this is an unauthenticated add; autoTMM routes the category to its save path.
# url + category are computed by the caller (see _autobrr-webhook.nix) and passed
# as call data; tags=manual marks these human-approved grabs.
{ config, ... }:
{
  qbt_add = {
    url = "http://${config.infra.clients.container-qbittorrent.ip}:${toString config.infra.clients.container-qbittorrent.ports.webui}/api/v2/torrents/add";
    method = "POST";
    content_type = "application/x-www-form-urlencoded";
    payload = "urls={{ url | urlencode }}&category={{ category }}&tags=manual";
  };
}
