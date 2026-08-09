# TL announces both TV and movies — split by category so each goes to the right
# *arr. match_categories uses wildcard semantics: a term without */?/| is an EXACT
# match, so "TV*" is needed to catch the "::"-hierarchy ("TV :: Foreign").
{ config, ... }:
{
  name = "TV";
  enabled = true;
  match_categories = "TV*";
  indexers = [ { id = "\${ref.indexer.TorrentLeech}"; } ];
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
