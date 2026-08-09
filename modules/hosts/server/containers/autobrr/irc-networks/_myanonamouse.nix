# MyAnonaMouse IRC network. autobrr only auto-derives a network from an indexer
# via the UI wizard, NOT the configuratarr API sync — so declare it explicitly,
# like TorrentLeech. name MUST match the indexer's irc.network ("MyAnonamouse")
# so announces link to it. NickServ auth (MAM requires identify to see #announce);
# nick = registered <user>|bot variant, account = base <user>.
_: {
  name = "MyAnonamouse";
  enabled = true;
  server = "irc.myanonamouse.net";
  port = 6697;
  tls = true;
  nick = "\${env.MAM_NICK}|bot";
  auth = {
    mechanism = "NICKSERV";
    account = "\${env.MAM_NICK}";
    password = "\${env.MAM_IRC_PASSWORD}";
  };
  channels = [ { name = "#announce"; } ];
}
