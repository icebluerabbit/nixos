# RetroToon — IRC-announce indexer. There is no upstream autobrr definition; ours
# is loaded from ../definitions/retrotoon.yaml via containerConfigs.autobrr
# .customDefinitions, so identifier MUST match that file's identifier.
# passkey is the same value Prowlarr uses (torznab apikey == torrent_pass).
# nick is a grouped variant of the account nick — the account, not the bot nick,
# is what NickServ authenticates. base_url is REQUIRED for irc indexers.
_: {
  name = "RetroToon";
  identifier = "retrotoon";
  implementation = "irc";
  enabled = true;
  base_url = "https://www.retrotoon.world/";
  settings = {
    passkey = "\${env.RETROTOON_PASSKEY}";
    nick = "\${env.RETROTOON_NICK}-autobrr";
    "auth.account" = "\${env.RETROTOON_NICK}";
    "auth.password" = "\${env.RETROTOON_IRC_PASSWORD}";
  };
}
