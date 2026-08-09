# Shared filter action: on every grab, POST the release to cross-seed's
# /api/announce so it searches the OTHER configured Torznab indexers for the
# same content and injects any cross-seed match. Fires from autobrr directly
# (it owns the release macros + the key) — no Home Assistant middleman.
# The apikey comes from CROSS_SEED_APIKEY in the mounted autobrr-env, so it
# never lands in the Nix store.
#
# The key goes in the URL query string, NOT an X-Api-Key header: configuratarr
# does not sync webhook_headers to autobrr, so a header-based key silently never
# reaches cross-seed and every announce 401s. webhook_host IS synced (and
# ${env.*} resolves in it), so ?apikey= is the reliable path. cross-seed accepts
# either form.
{ config, ... }:
let
  cs = config.infra.clients.container-cross-seed;
in
{
  name = "cross-seed";
  action_type = "WEBHOOK";
  enabled = true;
  webhook_host = "http://${cs.ip}:${toString cs.ports.default}/api/announce?apikey=\${env.CROSS_SEED_APIKEY}";
  webhook_method = "POST";
  webhook_type = "JSON";
  # cross-seed v6 announce payload (https://www.cross-seed.org/docs/v6-migration#autobrr-update).
  # v5 shape (.DownloadUrl/.InfoUrl/.Indexer) trips "Missing required params" — v6
  # wants .TorrentUrl for guid+link, the indexer *name* (.IndexerName, not the
  # .Indexer identifier) so it lines up with prowlarr for linkDirs, and .Cookie
  # so cross-seed can pull the .torrent from private trackers.
  webhook_data = builtins.toJSON {
    name = "{{ .TorrentName }}";
    guid = "{{ .TorrentUrl }}";
    link = "{{ .TorrentUrl }}";
    tracker = "{{ .IndexerName }}";
    cookie = "{{ .Cookie }}";
  };
  webhook_headers = [
    "Content-Type: application/json"
  ];
}
