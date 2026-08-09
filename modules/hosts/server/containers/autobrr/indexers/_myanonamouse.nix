# MyAnonaMouse — IRC-announce indexer. autobrr derives the IRC network (server,
# #announce channel, MouseBot announcer) and connects from THIS indexer, so the
# IRC login lives in settings — NOT a separate irc_networks entry (that would
# clobber the derived NickServ auth). base_url is REQUIRED for irc indexers.
# nick = registered variant <user>|bot; auth.account = base <user>.
# cookie = autobrr's own MAM session (mam-autobrr-id). mousetrap has a separate
# session (MAM rotates mam_id per request → one owner per session). Not exercised
# in P1 (only action is a webhook to mousetrap — nothing is downloaded yet).
_: {
  name = "MyAnonamouse";
  identifier = "myanonamouse";
  implementation = "irc";
  enabled = true;
  base_url = "https://www.myanonamouse.net";
  settings = {
    cookie = "mam_id=\${env.MAM_ID};";
    nick = "\${env.MAM_NICK}|bot";
    "auth.account" = "\${env.MAM_NICK}";
    "auth.password" = "\${env.MAM_IRC_PASSWORD}";
  };
}
