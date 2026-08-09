# Rizon — anime/fandom network; #kpop lives here (small). Nick and NickServ
# password are sops secrets (kept out of the Nix store); channels are not secret.
# IDENTIFY is sent as a command after connect, so joins race it.
{
  name = "Rizon";
  host = "irc.rizon.net";
  port = 6697;
  nickPath = "/run/secrets/rizon-username";
  channels = [
    "#chat"
    "#kpop"
  ];
  commands = [ "/msg NickServ IDENTIFY {nsPassword}" ];
  commandsSecretPaths = {
    nsPassword = "/run/secrets/rizon-nickserv-password";
  };
}
