_: {
  name = "PreToMe";
  identifier = "pretome";
  implementation = "irc";
  enabled = true;
  base_url = "https://pretome.info/";
  settings = {
    rsskey = "\${env.PRETOME_RSS_KEY}";
    nick = "\${env.PRETOME_NICK}";
    "auth.account" = "\${env.PRETOME_NICK}";
    "auth.password" = "\${env.PRETOME_IRC_PASSWORD}";
  };
}
