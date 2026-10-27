# PreToMe movies. Same wildcard semantics as _pretome-tv.nix — "Movies*" catches
# the pipe-delimited subcats ("Movies|x264", "Movies|XviD") without the pattern
# itself containing a pipe.
{ config, ... }:
{
  name = "PreToMe Movies";
  # Disabled — see indexers/_pretome.nix.
  enabled = false;
  match_categories = "Movies*";
  indexers = [ { id = "\${ref.indexer.PreToMe}"; } ];
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
