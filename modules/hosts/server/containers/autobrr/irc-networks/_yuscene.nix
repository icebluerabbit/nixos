# YU-Scene IRC network. autobrr only auto-derives a network from an indexer via
# the UI wizard, NOT the configuratarr API sync — so declare it explicitly, like
# TorrentLeech and MyAnonamouse. name MUST match the indexer definition's
# irc.network ("YU-Scene") so announces link to it.
#
# SASL_PLAIN, not NICKSERV and not NONE: the server rejects any connect that has
# not authenticated at the CAP stage (ACCOUNT_REQUIRED) and server PASS is
# disabled outright. The SASL account is the site username even though the nick
# is <username>_BOT — the site issues one account and the bot nick hangs off it.
# The password is the site irc_key, not the IRC/NickServ password; regenerating
# the key on the site drops every live session using the old one.
#
# ONE-TIME MANUAL STEP: after the bot first connects, run
# `/msg NickServ GROUP` from it to bind x6sic6x_BOT to the main account.
# NickServ has no idempotent path from Nix, so it is deliberately not seeded.
_: {
  name = "YU-Scene";
  enabled = true;
  server = "irc.yu-scene.net";
  port = 6697;
  tls = true;
  nick = "\${env.YUSCENE_NICK}_BOT";
  auth = {
    mechanism = "SASL_PLAIN";
    account = "\${env.YUSCENE_NICK}";
    password = "\${env.YUSCENE_IRCKEY}";
  };
  channels = [ { name = "#announce"; } ];
}
