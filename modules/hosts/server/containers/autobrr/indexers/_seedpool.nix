# SeedPool — IRC-announce indexer, definition shipped by autobrr (go:embed, so it
# is in the binary, not on disk — /api/indexer/schema is the way to read it).
# Requires autobrr >= v1.71.0 for the definition to exist at all.
#
# Only site credential the definition takes is the RSS key (RID), because it
# builds grabs itself from the announce: /torrent/download/{torrentId}.{rsskey}.
# NOT the API key (that is Prowlarr's, which gets a ready-made download_link from
# the UNIT3D API). The passkey IS needed here, but as the IRC *server password* —
# see irc-networks/_seedpool.nix — not as a grab credential.
#
# autobrr also ships a second definition, `seedpool_music`, which parses the
# richer #music announce. It is not configured, for two reasons: SeedPool is
# deliberately not wired into Lidarr at all (no music filter here, no music tag
# on the Prowlarr indexer), and it carries the SAME base_url as this one —
# autobrr maps an announce onto an indexer by base_url, so two indexers sharing
# it makes that mapping ambiguous.
#
# base_url is REQUIRED for irc indexers — autobrr rejects an empty one.
_: {
  name = "SeedPool";
  identifier = "seedpool";
  implementation = "irc";
  enabled = true;
  base_url = "https://seedpool.org/";
  settings = {
    rsskey = "\${env.SEEDPOOL_RSSKEY}";
    nick = "\${env.SEEDPOOL_NICK}";
    pass = "\${env.SEEDPOOL_PASSKEY}";
  };
}
