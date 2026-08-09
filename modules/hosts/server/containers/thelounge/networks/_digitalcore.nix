# DigitalCore IRC — ENDOR gates the private channels; the invite carries the
# username + IRC key (Settings -> Security), so both are sops secrets kept out of
# the Nix store. DC's setup guide has no NickServ step, so no IDENTIFY is seeded —
# add one (plus a digitalcore-irc-password secret) if the nick is registered later.
# The joins are send-order after the invite, not ack-waited.
{
  name = "DigitalCore";
  host = "irc.digitalcore.club";
  port = 7000; # SSL
  nickPath = "/run/secrets/digitalcore-username";
  channels = [ ];
  commands = [
    "/msg ENDOR !invite {username} {ircKey}"
    "/join #digitalcore"
  ];
  commandsSecretPaths = {
    username = "/run/secrets/digitalcore-username";
    ircKey = "/run/secrets/digitalcore-irc-key";
  };
}
