# DarkPeers movies. Split per-tracker like the DigitalCore filters: category naming
# is per-tracker and matching is prefix-anchored, so one match_categories cannot
# serve DP's flat "Movies" alongside DC's "Movies/2160p" or TL's "Movies :: *".
#
# Trailing * because a term without */?/| is an EXACT match: DP's shipped definition
# only proves the string "Movies" (both its test lines), so the wildcard also covers
# a "Movies/1080p"-style refinement. Tighten to an exact match once real announces
# confirm the spelling.
{ config, ... }:
{
  name = "DarkPeers Movies";
  enabled = true;
  match_categories = "Movies*";
  indexers = [ { id = "\${ref.indexer.DarkPeers}"; } ];
  actions = [
    {
      name = "Radarr";
      action_type = "RADARR";
      enabled = true;
      client_id = "\${ref.download_client.Radarr}";
    }
    (import ./_cross-seed-action.nix { inherit config; })
  ];
}
