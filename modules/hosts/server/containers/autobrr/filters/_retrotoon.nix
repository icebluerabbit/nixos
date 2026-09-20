# RetroToon categories (Retro/Modern/Anime/CGI/Feature) are era buckets, not media
# types — Balto.1995 announces as "Retro", Legend of the Galactic Heroes and a 2026
# film both announce as "Anime", and Wonder Pets S01 is "Modern" while S02/S03 are
# "Retro". No match_categories split is possible, so every announce goes to both
# *arrs and each rejects what it has no matching series/movie for.
{ config, ... }:
{
  name = "RetroToon";
  enabled = true;
  indexers = [ { id = "\${ref.indexer.RetroToon}"; } ];
  actions = [
    {
      name = "Sonarr";
      action_type = "SONARR";
      enabled = true;
      client_id = "\${ref.download_client.Sonarr}";
    }
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
