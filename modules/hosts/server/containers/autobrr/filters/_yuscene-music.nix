# YU-Scene music. "Music*" takes every music subcat, same as _pretome-music.nix.
#
# UNVERIFIED spelling, unlike the Movies/TV filters here: no shipped test line
# exercises a music announce, so "Music" comes from the site's category list
# (Prowlarr's definition maps id 8 to Audio/"Music"). Music is also not the
# tracker's focus — its own description is "MOVIES / TV" — so expect this one to
# fire rarely. There is a separate id 10 "Music Video" (Audio/Video) that this
# does NOT match, by design.
#
# NOTE: no freeleech gate here, and none is possible downstream — autobrr does not
# send indexer flags to Lidarr (internal/action/lidarr.go builds a lidarr.Release
# with no IndexerFlags field, unlike radarr.go/sonarr.go). Same limitation
# documented in _pretome-music.nix and _torrentleech-music.nix.
{ config, ... }:
{
  name = "YU-Scene Music";
  enabled = true;
  match_categories = "Music*";
  indexers = [ { id = "\${ref.indexer.YU-Scene}"; } ];
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
