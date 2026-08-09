# DarkPeers music. "Music*" takes every music subcat, same as _pretome-music.nix.
#
# UNVERIFIED spelling, like _darkpeers-tv.nix: no shipped test line exercises a music
# announce, so "Music" comes from the site's category list (Prowlarr's definition maps
# id 3 to Audio/"Music"). Music is also not the tracker's focus — its own description
# is "HD MOVIES / TV" — so expect this one to fire rarely.
#
# NOTE: no freeleech gate here, and none is possible downstream — autobrr does not
# send indexer flags to Lidarr (internal/action/lidarr.go builds a lidarr.Release
# with no IndexerFlags field, unlike radarr.go/sonarr.go). Same limitation
# documented in _pretome-music.nix and _torrentleech-music.nix.
{ config, ... }:
{
  name = "DarkPeers Music";
  enabled = true;
  match_categories = "Music*";
  indexers = [ { id = "\${ref.indexer.DarkPeers}"; } ];
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
