# DigitalCore IRC network. autobrr only auto-derives a network from an indexer via
# the UI wizard, NOT the configuratarr API sync — so declare it explicitly, like
# TorrentLeech and MyAnonamouse. name MUST match the indexer's irc.network
# ("DigitalCore") so announces link to it. Server/port/channel come from autobrr's
# shipped definition; ENDOR is the announcer and gates #announce behind the invite.
# No NickServ block: DC's setup guide has no registration step.
_: {
  name = "DigitalCore";
  enabled = true;
  server = "irc.digitalcore.club";
  port = 7000;
  tls = true;
  nick = "\${env.DIGITALCORE_NICK}-autobrr";
  invite_command = "ENDOR !invite \${env.DIGITALCORE_NICK} \${env.DIGITALCORE_IRC_KEY}";
  channels = [ { name = "#announce"; } ];
}
