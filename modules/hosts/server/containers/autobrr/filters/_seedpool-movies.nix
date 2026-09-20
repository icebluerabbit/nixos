# SeedPool movies. Split per-tracker like the DarkPeers/YU-Scene filters:
# category naming is per-tracker and matching is prefix-anchored, so one
# match_categories cannot serve SeedPool's flat "Movie" alongside DC's
# "Movies/2160p" or TL's "Movies :: *".
#
# Spelling is VERIFIED: the shipped definition's test lines announce
# 【 Movie 】, singular. The trailing * covers that and the plural, plus any
# later "Movie/1080p"-style refinement — a term without */?/| is an EXACT match
# in autobrr.
{ config, ... }:
{
  name = "SeedPool Movies";
  enabled = true;
  match_categories = "Movie*";
  indexers = [ { id = "\${ref.indexer.SeedPool}"; } ];
  actions = [
    {
      name = "Radarr";
      action_type = "RADARR";
      enabled = true;
      client_id = "\${ref.download_client.Radarr}";
    }
    (import ./_cross-seed-action.nix { inherit config; })
    (import ./_managarr-action.nix { inherit config; })
  ];
}
