# RetroToon — plain Torznab passthrough (torznab.php). No Cloudflare challenge on
# the endpoint, so no proxy tag. The definition's field is named `apikey`, but the
# value is the account passkey from https://www.retrotoon.world/prowlarr.php —
# hence the RETROTOON_PASSKEY secret. A stale value makes the site answer the
# connect test with 401, which fails the whole configuratarr apply.
_: {
  name = "RetroToon";
  implementation = "Cardigann";
  config_contract = "CardigannSettings";
  enable = true;
  redirect = false;
  priority = 30;
  protocol = "torrent";
  privacy = "private";
  app_profile_id = 1;
  tags = [ "\${ref.tag.tv}" ];
  fields = {
    definitionFile = "retrotoon";
    apikey = "\${env.RETROTOON_PASSKEY}";
  };
}
