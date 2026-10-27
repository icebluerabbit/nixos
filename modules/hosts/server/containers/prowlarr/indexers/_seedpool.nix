# SeedPool — UNIT3D JSON API indexer, same shape as DarkPeers and YU-Scene: auth
# is an Authorization: Bearer header, not a login form, so no scraping and no
# FlareSolverr tag. The definition ships a single site link, so baseUrl is left
# at its default. apikey is the site API token (My Settings -> API Key), NOT the
# passkey and NOT the RSS key — the API returns a ready-made download_link.
# definitionFile is the definition's FILE KEY (seedpool-api.yml); here it happens
# to match the `id:` inside it, as with DigitalCore.
#
# name is "SeedPool" rather than the definition's own "seedpool (API)" so it
# matches the autobrr indexer: _cross-seed-action.nix forwards {{ .IndexerName }}
# and cross-seed lines that up with Prowlarr for its per-tracker linkDirs.
_: {
  name = "SeedPool";
  implementation = "Cardigann";
  config_contract = "CardigannSettings";
  enable = true;
  redirect = false;
  # Second only to TorrentLeech (10), ahead of DigitalCore (20).
  priority = 15;
  protocol = "torrent";
  privacy = "private";
  app_profile_id = 1;
  # No music tag: SeedPool is not wired into Lidarr, so the indexer is not synced
  # there and there is no autobrr music filter either. cross-seed is unaffected —
  # it reads the Torznab feed directly and is not category-gated, so music we
  # already hold still cross-seeds here.
  tags = [
    "\${ref.tag.movies}"
    "\${ref.tag.tv}"
  ];
  fields = {
    definitionFile = "seedpool-api";
    apikey = "\${env.SEEDPOOL_APIKEY}";
  };
}
