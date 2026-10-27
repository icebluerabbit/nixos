# YU-Scene IRC — SASL PLAIN is mandatory and server PASS is disabled, so this is
# the first network here that authenticates at the CAP stage instead of with an
# on-connect NickServ command; a non-SASL connect is refused outright with
# ACCOUNT_REQUIRED. The SASL account is the site username (case-sensitive) and
# the SASL password is the site irc_key
# (https://yu-scene.net/users/<username>/irckeys) — NOT the IRC/NickServ
# password. The nick must equal the site username exactly. Regenerating the
# irc_key on the site drops every live session using the old one.
#
# #General is auto-joined by the server; it is declared anyway so the seeder owns
# the channel list. #support is joined by hand when needed, and #announce is left
# to autobrr on its own x6sic6x_BOT nick.
{
  name = "YU-Scene";
  host = "irc.yu-scene.net";
  port = 6697; # TLS required
  nickPath = "/run/secrets/yuscene-username";
  sasl = "plain";
  saslAccountPath = "/run/secrets/yuscene-username";
  saslPasswordPath = "/run/secrets/yuscene-irckey";
  channels = [ "#General" ];
}
