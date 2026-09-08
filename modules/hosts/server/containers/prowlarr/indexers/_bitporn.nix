# BitPorn — UNIT3D JSON API indexer, same shape as SeedPool, YU-Scene and
# DarkPeers: auth is an Authorization: Bearer header, not a login form, so no
# scraping and no FlareSolverr tag. The definition ships a single site link, so
# baseUrl is left at its default. apikey is the site API token (My Settings ->
# API Key), NOT the passkey — the API returns a ready-made download_link.
# Accounts are suspended after a month without a login.
#
# The definition exposes exactly one category (XXX, 6000), so this indexer
# carries only the xxx tag: app sync is tag-filtered, so no *arr picks it up.
# Nothing syncs that tag today; acquisition runs through autobrr.
#
# definitionFile is the definition's FILE KEY (bitporn.yml), which here matches
# the `id:` inside it, as with DigitalCore and SeedPool.
#
# No IRC: BitPorn runs no announce network, so autobrr polls it as a Torznab
# feed pointed back at this indexer — see autobrr/feeds/_bitporn.nix.
_: {
  name = "BitPorn";
  implementation = "Cardigann";
  config_contract = "CardigannSettings";
  enable = true;
  redirect = false;
  # Last: single-category XXX tracker, nothing else searches it.
  priority = 30;
  protocol = "torrent";
  privacy = "private";
  app_profile_id = 1;
  tags = [ "\${ref.tag.xxx}" ];
  fields = {
    definitionFile = "bitporn";
    apikey = "\${env.BITPORN_APIKEY}";
  };
}
