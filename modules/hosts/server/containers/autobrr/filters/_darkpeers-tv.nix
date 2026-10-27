# DarkPeers TV. Split per-tracker for the same reason as _darkpeers-movies.nix.
#
# UNVERIFIED spelling: unlike "Movies", no shipped test line exercises a TV announce,
# so "TV" comes from the site's own category list (Prowlarr's definition maps id 2 to
# TV). The trailing * covers "TV" and "TV/1080p", but NOT a lowercase "Tv" — which is
# exactly what DigitalCore announces, so the variant is plausible here too. If TV
# releases show up in autobrr's feed but never match, check the announced category
# string first.
{ config, ... }:
{
  name = "DarkPeers TV";
  enabled = true;
  match_categories = "TV*";
  indexers = [ { id = "\${ref.indexer.DarkPeers}"; } ];
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
