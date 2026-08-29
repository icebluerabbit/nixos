# SeedPool IRC — plain server PASS holding the site *passkey*, the same value
# that sits in every .torrent's announce URL. No NickServ and no SASL: the server
# has no separate IRC key, and the passkey is what identifies the session, so
# there is nothing to send on connect. Regenerating the passkey on the site
# invalidates this login along with every existing .torrent.
#
# The nick must equal the site username exactly — SeedPool's chat guide is
# explicit that a different nick is refused. autobrr sits on the same network
# under x6sic6x_bot per SeedPool's own autobrr guide, so the two clients do not
# collide; the server permits several connections on one identity regardless.
#
# #announce and #music are left to autobrr. #lobby is where !help lives and is
# joined by hand when needed.
{
  name = "SeedPool";
  host = "irc.seedpool.org";
  port = 6697; # TLS
  nickPath = "/run/secrets/seedpool-username";
  passwordPath = "/run/secrets/seedpool-passkey";
  channels = [ "#seedpool" ];
}
