{ config, ... }:
{
  name = "Movies";
  enabled = true;
  match_categories = "Movies*";
  indexers = [ { id = "\${ref.indexer.TorrentLeech}"; } ];
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
