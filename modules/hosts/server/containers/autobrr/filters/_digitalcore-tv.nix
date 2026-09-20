# DigitalCore TV. Separate from _tv.nix rather than adding DC to its indexers list:
# category naming is per-tracker. TL announces a "::" hierarchy ("TV :: Foreign"),
# DC announces "Tv/1080p", so one match_categories cannot serve both. Same wildcard
# semantics as _tv.nix — a term without */?/| is an EXACT match, hence the trailing *.
{ config, ... }:
{
  name = "DigitalCore TV";
  enabled = true;
  match_categories = "Tv/*";
  indexers = [ { id = "\${ref.indexer.DigitalCore}"; } ];
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
