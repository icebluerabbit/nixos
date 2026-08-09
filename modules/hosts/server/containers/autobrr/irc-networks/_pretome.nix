# PreToMe IRC network. autobrr only auto-derives a network from an indexer via the
# UI wizard, NOT the configuratarr API sync — so declare it explicitly, like the
# other trackers. name MUST match the definition's irc.network ("PreToMe") so
# announces link to it.
#
# invite_command is NOT in ptm.yaml: PreToMe moved #announce behind
# "/msg GateKeeper enter <IRCKEY>" (autobrr#2260), the maintainer acknowledged it
# and closed the issue without landing a definition change. Because the definition
# carries no invite_command setting, handler.go leaves the channel state machine's
# copy empty — but Handler.inviteCommand() still sends the network-level one after
# NickServ auth, so the grant happens; it just is not re-sent if the join fails.
_: {
  name = "PreToMe";
  enabled = true;
  server = "irc.pretome.info";
  port = 6697;
  tls = true;
  nick = "\${env.PRETOME_NICK}";
  auth = {
    mechanism = "NICKSERV";
    account = "\${env.PRETOME_NICK}";
    password = "\${env.PRETOME_IRC_PASSWORD}";
  };
  invite_command = "GateKeeper enter \${env.PRETOME_IRC_KEY}";
  channels = [ { name = "#announce"; } ];
}
