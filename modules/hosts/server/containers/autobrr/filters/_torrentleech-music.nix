# TL announces music under a "Music :: <sub>" hierarchy; "Music :: Audio*" takes the
# audio subcats only, so "Music :: Music videos" is never pushed to Lidarr. Same
# wildcard semantics as _tv.nix: a term without */?/| is an EXACT match.
#
# NOTE: no freeleech gate here, and none is possible downstream — autobrr does not
# send indexer flags to Lidarr (internal/action/lidarr.go builds a lidarr.Release with
# no IndexerFlags field, unlike radarr.go/sonarr.go, which call BuildIndexerFlags).
# A Lidarr custom format keyed on the freeleech flag therefore scores 0 on every push.
# The quality floor in quality-profiles/_any.nix is what gates this path.
{ config, ... }:
{
  name = "Music";
  enabled = true;
  match_categories = "Music :: Audio*";
  indexers = [ { id = "\${ref.indexer.TorrentLeech}"; } ];
  actions = [
    {
      name = "Lidarr";
      action_type = "LIDARR";
      enabled = true;
      client_id = "\${ref.download_client.Lidarr}";
    }
    (import ./_cross-seed-action.nix { inherit config; })
  ];
}
