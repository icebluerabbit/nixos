# Libera.Chat — largest public IRC network; the official #nixos channel lives
# here. Nick and NickServ password are sops secrets (kept out of the Nix store);
# channels are not secret. `##` is Libera's namespace for unaffiliated topic
# channels — `#` is reserved for project-claimed ones. IDENTIFY is sent as a
# command after connect, so joins race it; a `+r` channel that rejects the join
# would need SASL instead (the seeder has no SASL support today).
{
  name = "Libera.Chat";
  host = "irc.libera.chat";
  port = 6697;
  nickPath = "/run/secrets/libera-username";
  channels = [
    "#linux"
  ];
  commands = [ "/msg NickServ IDENTIFY {nsPassword}" ];
  commandsSecretPaths = {
    nsPassword = "/run/secrets/libera-nickserv-password";
  };
}
