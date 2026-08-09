# DarkPeers — UNIT3D JSON API indexer (auth is an Authorization: Bearer header,
# not a login form), so no scraping and no FlareSolverr tag. The definition ships
# a single site link, so baseUrl is left at its default. apikey is the site API
# token (Settings -> API Key), NOT the passkey and NOT the RSS key — the API
# returns a ready-made download_link, so neither of those is needed here.
# Accounts deactivate after 90 days without a login or seeding, and are pruned
# at 120.
# definitionFile is the definition's FILE KEY (darkpeers.yml), not the `id:` inside
# it (darkpeers-api) — Prowlarr fetches indexers.prowlarr.com/master/<ver>/<key> and
# 404s the whole configuratarr apply otherwise. DigitalCore hides the distinction:
# its file is digitalcore-api.yml, so key and id happen to match.
_: {
  name = "DarkPeers";
  implementation = "Cardigann";
  config_contract = "CardigannSettings";
  enable = true;
  redirect = false;
  priority = 25;
  protocol = "torrent";
  privacy = "private";
  app_profile_id = 1;
  tags = [
    "\${ref.tag.movies}"
    "\${ref.tag.tv}"
    "\${ref.tag.music}"
  ];
  fields = {
    definitionFile = "darkpeers";
    apikey = "\${env.DARKPEERS_APIKEY}";
  };
}
