# DigitalCore — IRC-announce indexer. Fields transcribed from autobrr's shipped
# definition (internal/indexer/definitions/digitalcore.yaml): passkey is the
# tracker passkey used to build the download URL
# (/api/v1/torrents/download/{torrentId}/{passkey}) — NOT the Prowlarr API key and
# NOT the IRC key, which is the usual cause of "announces parse but grabs fail".
# invite_command's USERNAME/IRCKEY placeholders take the account username and the
# IRC key. base_url is REQUIRED for irc indexers — autobrr maps the indexer into
# the announce handler by it. The bot nick must differ from the one thelounge
# holds on this network, or the two clients collide.
_: {
  name = "DigitalCore";
  identifier = "digitalcore";
  implementation = "irc";
  enabled = true;
  base_url = "https://digitalcore.club/";
  settings = {
    passkey = "\${env.DIGITALCORE_PASSKEY}";
    nick = "\${env.DIGITALCORE_NICK}-autobrr";
    invite_command = "ENDOR !invite \${env.DIGITALCORE_NICK} \${env.DIGITALCORE_IRC_KEY}";
  };
}
