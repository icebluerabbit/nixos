# DigitalCore movies. Split from _movies.nix for the same reason as
# _digitalcore-tv.nix — DC announces "Movies/2160p", TL announces "Movies*" under a
# "::" hierarchy. Matching is prefix-anchored, so "Movies/*" does NOT catch DC's
# XXX/Movies/* tree.
{ config, ... }:
{
  name = "DigitalCore Movies";
  enabled = true;
  match_categories = "Movies/*";
  indexers = [ { id = "\${ref.indexer.DigitalCore}"; } ];
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
