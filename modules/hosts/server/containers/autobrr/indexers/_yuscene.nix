# YU-Scene — IRC-announce indexer, definition shipped by autobrr (yu-scene.yaml).
# Only credential the definition takes is the RSS key, because it builds grabs
# itself from the announce: /torrent/download/{torrentId}.{rsskey}. NOT the API
# key (that is Prowlarr's, which gets a ready-made download_link from the UNIT3D
# API) and NOT the passkey (that is the tracker announce, already inside the
# .torrent).
# Unlike DarkPeers, the bot cannot connect unauthenticated: the server refuses
# every non-SASL connect with ACCOUNT_REQUIRED, so auth.account/auth.password are
# mandatory here. account is the site username and password is the site irc_key —
# the same pair thelounge uses, because SASL is the only credential the network
# accepts.
# The bot nick is <username>_BOT, which is the site's documented convention and
# also keeps it off the nick thelounge holds. It must be grouped to the main
# account once by hand (see irc-networks/_yuscene.nix).
# base_url is REQUIRED for irc indexers — autobrr maps announces to the indexer
# by it. Name matches Prowlarr's ("YU-Scene") because _cross-seed-action.nix
# forwards {{ .IndexerName }} and cross-seed lines that up with Prowlarr.
_: {
  name = "YU-Scene";
  identifier = "yu-scene";
  implementation = "irc";
  enabled = true;
  base_url = "https://yu-scene.net/";
  settings = {
    rsskey = "\${env.YUSCENE_RSSKEY}";
    nick = "\${env.YUSCENE_NICK}_BOT";
    "auth.account" = "\${env.YUSCENE_NICK}";
    "auth.password" = "\${env.YUSCENE_IRCKEY}";
  };
}
