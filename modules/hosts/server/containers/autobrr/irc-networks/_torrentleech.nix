_: {
  name = "TorrentLeech.org";
  enabled = true;
  server = "irc.torrentleech.org";
  port = 7021;
  tls = true;
  nick = "\${env.TORRENTLEECH_NICK}_bot";
  channels = [ { name = "#tlannounces"; } ];
}
