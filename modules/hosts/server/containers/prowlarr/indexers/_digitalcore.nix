# DigitalCore — JSON API indexer (auth is an X-API-KEY header, not a login form),
# so no scraping and no FlareSolverr tag. The definition ships two links; baseUrl
# pins the main site — switch to https://prxy.digitalcore.club/ only if that host
# becomes unreachable. Account is disabled after 90 days without a login.
_: {
  name = "DigitalCore";
  implementation = "Cardigann";
  config_contract = "CardigannSettings";
  enable = true;
  redirect = false;
  priority = 20;
  protocol = "torrent";
  privacy = "private";
  app_profile_id = 1;
  tags = [
    "\${ref.tag.movies}"
    "\${ref.tag.tv}"
    "\${ref.tag.music}"
  ];
  fields = {
    definitionFile = "digitalcore-api";
    baseUrl = "https://digitalcore.club/";
    apikey = "\${env.DIGITALCORE_APIKEY}";
  };
}
