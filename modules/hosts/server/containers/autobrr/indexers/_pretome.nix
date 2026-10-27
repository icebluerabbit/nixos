_: {
  name = "PreToMe";
  identifier = "pretome";
  implementation = "irc";
  # Disabled 2026-08-11: PreToMe is ~99% RARed scene releases. Kept declared
  # because configuratarr runs with prune = false (modules/containers/autobrr.nix:66)
  # — removing the import would leave it enabled in autobrr's DB.
  enabled = false;
  base_url = "https://pretome.info/";
  settings = {
    rsskey = "\${env.PRETOME_RSS_KEY}";
    nick = "\${env.PRETOME_NICK}";
    "auth.account" = "\${env.PRETOME_NICK}";
    "auth.password" = "\${env.PRETOME_IRC_PASSWORD}";
  };
}
