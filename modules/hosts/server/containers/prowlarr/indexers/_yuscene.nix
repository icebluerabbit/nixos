# YU-Scene — UNIT3D JSON API indexer, same shape as DarkPeers: auth is an
# Authorization: Bearer header, not a login form, so no scraping and no
# FlareSolverr tag. The definition ships a single site link, so baseUrl is left
# at its default. apikey is the site API token (Settings -> API Key), NOT the
# passkey and NOT the RSS key — the API returns a ready-made download_link.
# Accounts are disabled after 90 days without a login or seeding, and become
# eligible for pruning 275 days after that.
# definitionFile is the definition's FILE KEY (yuscene-api.yml); here it happens
# to match the `id:` inside it. The bare key "yuscene" 404s.
_: {
  name = "YU-Scene";
  implementation = "Cardigann";
  config_contract = "CardigannSettings";
  enable = true;
  redirect = false;
  priority = 26;
  protocol = "torrent";
  privacy = "private";
  app_profile_id = 1;
  tags = [
    "\${ref.tag.movies}"
    "\${ref.tag.tv}"
    "\${ref.tag.music}"
  ];
  fields = {
    definitionFile = "yuscene-api";
    apikey = "\${env.YUSCENE_APIKEY}";
  };
}
