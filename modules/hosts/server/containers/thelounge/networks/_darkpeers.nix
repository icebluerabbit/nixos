# DarkPeers IRC — two-step gate: NickServ IDENTIFY, then `!verify <IRC key>` to the
# DP bot. Both are required; identifying alone leaves you in no channels. The nick
# was REGISTERed by hand once from this connection (NickServ has no self-service
# path from Nix) — re-registration is not idempotent, so it is deliberately not
# seeded here. The IRC key comes from the site: profile picture -> Settings -> IRC
# Key. RECOVER runs first so a ghost from a dropped connection can't strand us on a
# suffixed nick, which would then fail the verify.
#
# channels is empty on purpose: !verify auto-joins by user group (#darkpeers today),
# so the bot decides membership, not us. #announce and #pre are joined by hand if
# wanted — autobrr covers #announce on its own separate, unregistered nick.
{
  name = "DarkPeers";
  host = "irc.darkpeers.org";
  port = 6697; # SSL only
  nickPath = "/run/secrets/darkpeers-username";
  channels = [ ];
  commands = [
    "/msg NickServ RECOVER {nick} {nsPassword}"
    "/nick {nick}"
    "/msg NickServ IDENTIFY {nsPassword}"
    "/msg DP !verify {ircKey}"
  ];
  commandsSecretPaths = {
    nick = "/run/secrets/darkpeers-username";
    nsPassword = "/run/secrets/darkpeers-irc-password";
    ircKey = "/run/secrets/darkpeers-irckey";
  };
}
