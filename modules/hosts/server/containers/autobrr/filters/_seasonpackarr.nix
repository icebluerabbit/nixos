# Season packs whose episodes we already hold. seasonpackarr hardlinks those
# episodes into the pack folder and autobrr adds the pack, so the pack seeds at
# ~100% without re-downloading anything.
#
# `episodes = "0"` is autobrr's way of saying "no episode number" — i.e. a whole
# season — and `seasons = "1-99"` keeps it to season releases rather than a
# complete-series pack.
#
# Priority must outrank every TV filter: autobrr walks filters in priority order
# and stops at the first one whose actions all ran (release/service.go). The
# other filters in this directory leave priority unset (0), so 100 is ample.
#
# Falling through is the designed failure mode, not an error. /api/pack returns
# a non-250 whenever nothing matches — no episodes held, below smartMode's
# threshold, or a TVMaze lookup failure — and configuratarr's external check
# treats that as "filter did not match", so the announce continues to the normal
# TV filter and Sonarr grabs the pack the usual way.
#
# No resolution constraint on purpose: seasonpackarr compares resolution itself
# before anything else, so a 1080p pack can never match 2160p episodes. Pinning
# the filter to 1080p would just make 2160p packs invisible.
{ config, ... }:
let
  spa = config.infra.clients.container-seasonpackarr;
  endpoint =
    path:
    "http://${spa.ip}:${toString spa.ports.default}/api/${path}?apikey=\${env.SEASONPACKARR_API_TOKEN}";
in
{
  name = "Seasonpackarr";
  enabled = true;
  priority = 100;
  seasons = "1-99";
  episodes = "0";
  indexers = [
    { id = "\${ref.indexer.TorrentLeech}"; }
    { id = "\${ref.indexer.DigitalCore}"; }
    { id = "\${ref.indexer.PreToMe}"; }
    { id = "\${ref.indexer.DarkPeers}"; }
    { id = "\${ref.indexer.RetroToon}"; }
  ];
  # Match gate only: no filesystem writes, nothing added to the client. 250 is
  # seasonpackarr's own "matched" status, not an HTTP convention.
  external = [
    {
      name = "seasonpackarr-match";
      external_type = "WEBHOOK";
      enabled = true;
      webhook_host = endpoint "pack";
      webhook_method = "POST";
      webhook_expect_status = 250;
      webhook_data = builtins.toJSON {
        name = "{{ .TorrentName }}";
        clientname = "default";
      };
    }
  ];
  # Order matters: the parse webhook creates the hardlinks and must land before
  # qBittorrent is told to add the torrent, or the client finds an empty folder
  # and downloads the whole pack.
  actions = [
    {
      name = "Seasonpackarr Parse";
      action_type = "WEBHOOK";
      enabled = true;
      webhook_host = endpoint "parse";
      webhook_method = "POST";
      webhook_type = "JSON";
      # TorrentDataRawBytes is the .torrent itself — seasonpackarr parses it for
      # the real folder and file names rather than trusting the announce name.
      webhook_data = builtins.toJSON {
        name = "{{ .TorrentName }}";
        torrent = "{{ .TorrentDataRawBytes | js }}";
        clientname = "default";
      };
      webhook_headers = [
        "Content-Type: application/json"
      ];
    }
    {
      name = "qBittorrent Season Pack";
      action_type = "QBITTORRENT";
      enabled = true;
      client_id = "\${ref.download_client.qBittorrent}";
      category = "seasonpacks";
      # Must stay false. The hash check is what lets qBittorrent recognise the
      # hardlinked episodes as already-complete pieces; skipping it leaves the
      # torrent errored and needing a manual recheck.
      skip_hash_check = false;
    }
  ];
}
