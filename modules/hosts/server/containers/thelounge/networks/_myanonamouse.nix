# MyAnonaMouse IRC network for thelounge. Nick = site username; identify to
# NickServ (IRC password, which MAM requires differ from the site password),
# then join main + members (send-order, not ack-waited). Secrets are sops,
# kept out of the Nix store. #am-members requires auth → joined post-identify.
{
  name = "MyAnonaMouse";
  host = "irc.myanonamouse.net";
  port = 6697; # SSL only (7000 also SSL)
  nickPath = "/run/secrets/mam-username";
  channels = [ ];
  commands = [
    "/msg NickServ IDENTIFY {nsPassword}"
    "/join #am-members"
  ];
  commandsSecretPaths.nsPassword = "/run/secrets/mam-irc-password";
}
