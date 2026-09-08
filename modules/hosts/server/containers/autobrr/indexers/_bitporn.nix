# BitPorn — feed indexer, not IRC. BitPorn runs no announce network and autobrr
# ships no definition for it (only `pornbay` among the adult trackers), so this
# instantiates autobrr's generic `torznab` definition and points it back at
# Prowlarr's BitPorn indexer (id 8).
#
# Going through Prowlarr rather than the site's own RSS keeps the BitPorn API
# key in exactly one place (prowlarr-env), lets the shipped Cardigann definition
# absorb base-url/auth changes, and hands autobrr real Torznab categories — a
# bare RSS feed would leave every rule as a name regex, since there is no
# definition to parse categories or freeleech out of it.
#
# base_url is only required for irc indexers, so it is omitted here; the feed
# URL lives in settings (and again on the feed itself — autobrr stores them
# separately). See feeds/_bitporn.nix.
{ config, ... }:
let
  prowlarr = config.infra.clients.container-prowlarr;
in
{
  name = "BitPorn";
  identifier = "torznab";
  implementation = "torznab";
  enabled = true;
  settings = {
    url = "http://${prowlarr.ip}:${toString prowlarr.ports.default}/8/api";
    api_key = "\${env.PROWLARR_APIKEY}";
  };
}
