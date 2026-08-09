# TorrentLeech IRC network for thelounge. Identify to NickServ, then ask TL-Monkey
# to invite us to the members-only channel (send-order, not ack-waited). Both the
# password and IRC key are sops secrets, kept out of the Nix store.
{
  name = "TorrentLeech";
  host = "irc.torrentleech.org";
  port = 7021; # SSL (7011 non-SSL)
  nickPath = "/run/secrets/torrentleech-username";
  channels = [ ];
  commands = [
    "/msg NickServ IDENTIFY {nsPassword}"
    "/msg TL-Monkey !invite {inviteKey}"
  ];
  commandsSecretPaths = {
    nsPassword = "/run/secrets/torrentleech-irc-password";
    inviteKey = "/run/secrets/torrentleech-irc-key";
  };
}
