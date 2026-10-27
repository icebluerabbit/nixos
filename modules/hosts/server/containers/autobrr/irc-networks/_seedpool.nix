# SeedPool IRC network. autobrr only auto-derives a network from an indexer via
# the UI wizard, NOT the configuratarr API sync — so declare it explicitly, like
# TorrentLeech and MyAnonamouse. name MUST match the definition's irc.network
# ("SeedPool") so announces link to it.
#
# Auth is a plain server PASS holding the site *passkey* — the same value that is
# already inside every .torrent's announce URL. There is no separate IRC key and
# no NickServ: mechanism NONE is set explicitly rather than omitted, because
# DigitalCore's aspect omits the block and its stored network came back as
# NICKSERV.
#
# Nick is the bare site username. `x6sic6x_bot`, as SeedPool's autobrr guide
# says to use, was answered with `464 Password incorrect` on the same passkey;
# the bare nick got `001 Welcome`.
_: {
  name = "SeedPool";
  enabled = true;
  server = "irc.seedpool.org";
  port = 6697;
  tls = true;
  nick = "\${env.SEEDPOOL_NICK}";
  pass = "\${env.SEEDPOOL_PASSKEY}";
  auth = {
    mechanism = "NONE";
  };
  # SeedServ announces here. #music carries the same releases with richer
  # metadata but belongs to the seedpool_music definition, which is not
  # configured — see indexers/_seedpool.nix.
  channels = [ { name = "#announce"; } ];
}
