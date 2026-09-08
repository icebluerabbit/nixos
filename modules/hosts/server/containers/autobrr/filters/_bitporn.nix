# BitPorn — adds straight to qBittorrent under the `xxx` category, which maps to
# /mnt/media/complete/xxx. There is no *arr in this path: Whisparr was removed
# after it proved unable to match scene releases (title parser gives up on
# `[Site] YY.MM.DD Performer [1080p]` names), and Stash indexes complete/xxx
# directly, so identification no longer depends on an acquirer matching first.
#
# Same shape as the seasonpackarr filter — the other place autobrr adds to
# qBittorrent itself rather than handing off.
#
# Releases arrive from a Torznab feed rather than IRC (BitPorn runs no announce
# network) — see ../feeds/_bitporn.nix.
#
# No freeleech gate here on purpose: BitPorn's Cardigann definition reports only
# PER-TORRENT freeleech, so during a site-wide freeleech event 87 of 100
# releases still came through as downloadvolumefactor=1. Gating on it would drop
# almost everything while everything was in fact free.
{ config, ... }:
{
  name = "BitPorn";
  # DISABLED, and it must stay that way until something gates it. There are no
  # criteria here — no size, resolution or title match — so enabling it grabs
  # EVERY announce on the tracker (84 arrived in the first few hours, one of
  # them 19 GB). Whisparr used to be the gate by rejecting almost everything;
  # with it gone, the gate is the matching service that replaces it.
  enabled = false;
  indexers = [ { id = "\${ref.indexer.BitPorn}"; } ];
  actions = [
    {
      name = "qBittorrent";
      action_type = "QBITTORRENT";
      enabled = true;
      client_id = "\${ref.download_client.qBittorrent}";
      category = "xxx";
    }
    (import ./_cross-seed-action.nix { inherit config; })
  ];
}
