{ config, ... }:
{
  name = "qBittorrent";
  client_type = "qbittorrent";
  enabled = true;
  host = config.infra.clients.container-qbittorrent.ip;
  port = config.infra.clients.container-qbittorrent.ports.webui;
  # No credentials: the media VLAN is in qbt's authSubnetWhitelist, same as every
  # other *arr download-client entry here.
  #
  # Required, not optional. mousetrap only delivers to qBittorrent itself on the
  # announce path; a torznab grab returns the .torrent bytes and the downstream
  # client adds them ("mousetrap never adds it here"). Without this, proactive
  # search finds releases Bindery cannot act on.
  #
  # Categories match the qbt save paths, so completed downloads land under
  # /mnt/media/complete/{books,audiobooks} for Bindery to hardlink out of.
  category = "books";
  category_audiobook = "audiobooks";
  priority = 1;
}
