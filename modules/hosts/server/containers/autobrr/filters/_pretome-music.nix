# PreToMe music. "Music*" takes every music subcat, unlike TorrentLeech's narrower
# "Music :: Audio*" — PreToMe's category has no separate music-video branch to
# exclude.
#
# NOTE: no freeleech gate here, and none is possible downstream — autobrr does not
# send indexer flags to Lidarr (internal/action/lidarr.go builds a lidarr.Release
# with no IndexerFlags field, unlike radarr.go/sonarr.go). Same limitation
# documented in _torrentleech-music.nix.
{ config, ... }:
{
  name = "PreToMe Music";
  # Disabled — see indexers/_pretome.nix.
  enabled = false;
  match_categories = "Music*";
  indexers = [ { id = "\${ref.indexer.PreToMe}"; } ];
  actions = [
    {
      name = "Lidarr";
      action_type = "LIDARR";
      enabled = true;
      client_id = "\${ref.download_client.Lidarr}";
    }
    (import ./_cross-seed-action.nix { inherit config; })
    (import ./_managarr-action.nix { inherit config; })
  ];
}
