# RetroToon IRC — no announce bot and no invite flow, just #general. Idle time in
# the channel earns tracker bonus points, so the persistent connection is the
# point. The nick must be REGISTERed with NickServ manually once (from thelounge)
# before this IDENTIFY does anything; use the tracker username to earn the bonus.
{
  name = "RetroToon";
  host = "irc.retrotoon.world";
  port = 6697; # SSL (6667 plaintext)
  nickPath = "/run/secrets/retrotoon-username";
  channels = [ "#general" ];
  commands = [ "/msg NickServ IDENTIFY {nsPassword}" ];
  commandsSecretPaths = {
    nsPassword = "/run/secrets/retrotoon-irc-password";
  };
}
