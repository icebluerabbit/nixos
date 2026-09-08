# BitPorn Torznab feed — the polling counterpart to indexers/_bitporn.nix, and
# the only feed here: every other tracker announces over IRC, which is push and
# needs no polling at all.
#
# Each poll is a Torznab `t=search` with an empty term, so it is a real query
# against BitPorn through Prowlarr — not a static RSS document — and counts
# toward the indexer's query limit. 5 minutes is 288/day, the floor worth paying
# for a tracker with no announce network. categories pins the fetch to XXX
# (6000) — the only
# category the BitPorn definition exposes — so a definition change that adds
# more cannot silently widen what this feed pulls.
{ config, ... }:
let
  prowlarr = config.infra.clients.container-prowlarr;
in
{
  name = "BitPorn";
  indexer_id = "\${ref.indexer.BitPorn}";
  feed_type = "TORZNAB";
  enabled = true;
  url = "http://${prowlarr.ip}:${toString prowlarr.ports.default}/8/api";
  api_key = "\${env.PROWLARR_APIKEY}";
  interval = 5;
  categories = [ 6000 ];
}
