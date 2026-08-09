# DarkPeers IRC network. autobrr only auto-derives a network from an indexer via
# the UI wizard, NOT the configuratarr API sync — so declare it explicitly, like
# TorrentLeech and MyAnonamouse. name MUST match the indexer's irc.network
# ("DarkPeers") so announces link to it.
#
# mechanism NONE is set explicitly, per DP's own autobrr guide: the bot connects
# unregistered and #announce lets it in. Do NOT leave auth off entirely and expect
# a no-auth default — DigitalCore's aspect omits the block and its stored network
# came back as NICKSERV. No invite_command either: DP's !verify is the human path
# and would drag the bot into #darkpeers.
_: {
  name = "DarkPeers";
  enabled = true;
  server = "irc.darkpeers.org";
  port = 6697;
  tls = true;
  nick = "\${env.DARKPEERS_NICK}_autobrr";
  auth = {
    mechanism = "NONE";
  };
  channels = [ { name = "#announce"; } ];
}
