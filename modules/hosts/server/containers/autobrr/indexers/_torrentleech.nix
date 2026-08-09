# TorrentLeech: rss key drives the download URL (NOT the passkey/irc key — those
# don't work for autobrr). nick is <username>_bot with no NickServ (bot nicks are
# unregistered). base_url is REQUIRED for irc indexers — autobrr maps the indexer
# into the announce handler by it; without it the indexer is skipped and announces
# are never parsed.
_: {
  name = "TorrentLeech";
  identifier = "torrentleech";
  implementation = "irc";
  enabled = true;
  base_url = "https://www.torrentleech.org";
  settings = {
    rsskey = "\${env.TORRENTLEECH_RSS_KEY}";
    nick = "\${env.TORRENTLEECH_NICK}_bot";
  };
}
