# PreToMe announces a pipe-delimited category ("TV|x264", "Movies|XviD",
# "XXX|HiDef"). The pipe is in the announced value, not in this pattern — autobrr
# only treats */?/| as wildcards inside the filter's own pattern — so a plain "TV*"
# prefix is safe. XXX is most of PreToMe's volume and matches none of these.
{ config, ... }:
{
  name = "PreToMe TV";
  # Disabled — see indexers/_pretome.nix.
  enabled = false;
  match_categories = "TV*";
  indexers = [ { id = "\${ref.indexer.PreToMe}"; } ];
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
