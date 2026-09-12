# BitPorn Torznab feed — the polling counterpart to indexers/_bitporn.nix, and
# the only feed here: every other tracker announces over IRC, which is push and
# needs no polling at all.
#
# Each poll is a Torznab `t=search` with an empty term, so it is a real query
# against BitPorn through Prowlarr — not a static RSS document — and counts
# toward the indexer's query limit. categories pins the fetch to XXX (6000) —
# the only category the BitPorn definition exposes — so a definition change that
# adds more cannot silently widen what this feed pulls.
#
# The interval is a ratio decision rather than a freshness one. Upload comes
# from the initial leech burst on a new release, so a poll that arrives five
# minutes late joins a swarm other people are already seeding. Two minutes is
# 720 queries a day against a tracker with no announce network — more than the
# 288 this used to cost, and still a small fraction of what an IRC-fed tracker
# would deliver for free. Worth revisiting if BitPorn ever complains about
# query volume; the released torrents would still all be seen either way.
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
  interval = 2;
  categories = [ 6000 ];
}
