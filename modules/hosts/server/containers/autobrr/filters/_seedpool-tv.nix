# SeedPool TV. Split per-tracker for the same reason as _seedpool-movies.nix.
#
# Two terms, comma-separated (autobrr ORs them). "BoxSet*" is VERIFIED — the
# shipped definition's first test line announces 【 BoxSet 】 for a season pack.
# "TV*" is NOT verified by any shipped test line; Prowlarr's seedpool-api
# definition maps the site's TV category with the description "TV Show", so the
# wildcard covers both that and a bare "TV".
#
# Anime is deliberately absent: seedpool-api maps that category to BOTH TV/Anime
# and Movies/Other, so it cannot be routed to Sonarr or Radarr on the announce
# category alone. Add it to whichever side turns out to dominate.
{ config, ... }:
{
  name = "SeedPool TV";
  enabled = true;
  match_categories = "TV*,BoxSet*";
  indexers = [ { id = "\${ref.indexer.SeedPool}"; } ];
  actions = [
    {
      name = "Sonarr";
      action_type = "SONARR";
      enabled = true;
      client_id = "\${ref.download_client.Sonarr}";
    }
    (import ./_cross-seed-action.nix { inherit config; })
    (import ./_managarr-action.nix { inherit config; })
  ];
}
