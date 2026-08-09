# Snoonet — Reddit-adjacent network; hosts the largest #kpop channel left on IRC
# (~26 users, still an order of magnitude below the subreddit Discords). Nick is
# a sops secret (kept out of the Nix store); channels are not secret.
# Unregistered for now; NickServ IDENTIFY is added after manual registration.
{
  name = "Snoonet";
  host = "irc.snoonet.org";
  port = 6697;
  nickPath = "/run/secrets/snoonet-username";
  channels = [
    "#kpop"
    "#trackers"
    "#linux"
  ];
}
