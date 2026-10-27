# DarkPeers — IRC-announce indexer, definition shipped by autobrr (go:embed, so it
# is in the binary, not on disk — /api/indexer/schema is the way to read it). Only
# credential it takes is the RSS key, because it builds grabs itself from the
# announce: /torrent/download/{torrentId}.{rsskey}. NOT the API key (that is
# Prowlarr's, which gets a ready-made download_link from the UNIT3D API) and NOT
# the passkey (that is the tracker announce, already inside the .torrent).
# No auth.* settings: DP's #announce is open to any connected nick, so the bot
# skips both NickServ and the DP !verify gate — verifying would also auto-join it
# to #darkpeers, which it has no business sitting in. The bot nick must differ from
# the one thelounge holds, or the two clients collide.
# base_url is REQUIRED for irc indexers — autobrr maps announces to the indexer by it.
_: {
  name = "DarkPeers";
  identifier = "darkpeers";
  implementation = "irc";
  enabled = true;
  base_url = "https://darkpeers.org/";
  settings = {
    rsskey = "\${env.DARKPEERS_RSSKEY}";
    nick = "\${env.DARKPEERS_NICK}_autobrr";
  };
}
