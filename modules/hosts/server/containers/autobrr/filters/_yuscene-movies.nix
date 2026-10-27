# YU-Scene movies. Split per-tracker like the DarkPeers/DigitalCore filters:
# category naming is per-tracker and matching is prefix-anchored, so one
# match_categories cannot serve YU-Scene's flat "Movies" alongside DC's
# "Movies/2160p" or TL's "Movies :: *".
#
# Spelling is VERIFIED: the shipped definition's test lines announce
# `Category: [Movies]`. The trailing * is kept anyway so a later
# "Movies/1080p"-style refinement keeps matching — a term without */?/| is an
# EXACT match in autobrr.
{ config, ... }:
{
  name = "YU-Scene Movies";
  enabled = true;
  match_categories = "Movies*";
  indexers = [ { id = "\${ref.indexer.YU-Scene}"; } ];
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
