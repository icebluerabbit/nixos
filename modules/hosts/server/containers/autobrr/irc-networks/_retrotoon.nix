# RetroToon IRC network. autobrr only auto-derives a network from an indexer via
# the UI wizard, NOT the configuratarr API sync — so declare it explicitly, like
# TorrentLeech and MyAnonamouse. name MUST match the indexer's irc.network
# ("RetroToon") so announces link to it. NickServ auth against the base account,
# with the bot nick grouped to it.
_: {
  name = "RetroToon";
  enabled = true;
  server = "irc.retrotoon.world";
  port = 6697;
  tls = true;
  nick = "\${env.RETROTOON_NICK}-autobrr";
  auth = {
    mechanism = "NICKSERV";
    account = "\${env.RETROTOON_NICK}";
    password = "\${env.RETROTOON_IRC_PASSWORD}";
  };
  channels = [ { name = "#announce"; } ];
}
