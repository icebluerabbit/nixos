# YU-Scene TV. Split per-tracker for the same reason as _yuscene-movies.nix.
#
# Spelling is VERIFIED, unlike the DarkPeers equivalent: the shipped definition
# carries a test line announcing `Category: [TV]`. The trailing * covers "TV" and
# a later "TV/1080p"-style refinement.
{ config, ... }:
{
  name = "YU-Scene TV";
  enabled = true;
  match_categories = "TV*";
  indexers = [ { id = "\${ref.indexer.YU-Scene}"; } ];
  actions = [
    {
      name = "Sonarr";
      action_type = "SONARR";
      enabled = true;
      client_id = "\${ref.download_client.Sonarr}";
    }
    (import ./_cross-seed-action.nix { inherit config; })
  ];
}
