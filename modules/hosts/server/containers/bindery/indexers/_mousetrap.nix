{ config, ... }:
{
  name = "MyAnonaMouse";
  # Bindery's indexerTypeForProtocol yields "torznab" for torrent indexers and
  # "newznab" for usenet.
  indexer_type = "torznab";
  url = "http://${config.infra.clients.container-mousetrap.ip}:${toString config.infra.clients.container-mousetrap.ports.default}/torznab/api";
  # mousetrap runs with torznab-apikey empty, and torznabAuthorized short-circuits
  # to true when the configured key is empty. Reachable only from the media VLAN.
  api_key = "";
  # mousetrap's mainCatsFor maps 3000|3030 → MAM main-cat 13 (AudioBooks) and
  # 7000|7020 → 14 (E-Books); every other id falls through to both. So this list
  # is the whole addressable space — 7030 would not narrow comics, it would just
  # search everything and let Layer-1 filter.
  categories = [
    7000
    7020
    3030
  ];
  include_parent_categories = false;
  enabled = true;
  supports_search = true;
  priority = 25;
  # Off deliberately. Every result's download link points back at
  # /torznab/download, which re-runs Layer-1 accept and the Layer-2 grab gate
  # (unsat margin, buffer floor, personal-freeleech wedges) before fetching.
  # Gating again on this side would hide releases mousetrap would have wedged
  # through. Expect routine grab failures here: a held grab answers 404 by
  # design, so the consumer marks the release failed and moves on.
  freeleech_only = false;
}
