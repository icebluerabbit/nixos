# qbit-manage: tag-only maintenance (noHL + per-tracker + tracker-error tags).
# Purely labels torrents for manual review in the WebUI — every destructive
# command (rem_*/share_limits) stays false, so no removals, no H&R risk.
# noHL is hardlink-count based; cross-seeded torrents keep a link and won't
# flag (cross-seed tags its own injects, so those stay visible separately).
{
  lib,
  categories,
  webuiPort,
}:
{
  enable = true;
  scheduleMinutes = 1440;
  settings = {
    commands = {
      tag_update = true; # per-tracker tags (below)
      tag_tracker_error = true; # tag unregistered / not-working (no removal)
      tag_nohardlinks = true; # the noHL tag
      rem_unregistered = false;
      rem_orphaned = false;
      share_limits = false;
      cat_update = false;
      recheck = false;
    };
    # localhost + WebUILocalHostAuth=false → auth bypassed, no creds needed.
    qbt = {
      host = "localhost:${toString webuiPort}";
      user = "";
      pass = "";
    };
    directory.root_dir = "/mnt/media/";
    # Mandatory: category -> save path (must match qbt's own category paths).
    cat = categories;
    # Keys are substring-matched against the announce URL and split on "|", so one
    # entry covers a tracker's alternate announce domains (TL's tleechreload, PreToMe's
    # preto.me) — without them those torrents fall through to no tag at all.
    tracker = {
      "torrentleech|tleechreload".tag = "tracker-TL";
      "pretome|preto.me".tag = "tracker-PTM";
      "myanonamouse".tag = "tracker-MAM";
      "digitalcore".tag = "tracker-DC";
      "retrotoon".tag = "tracker-RTW";
      "darkpeers".tag = "tracker-DP";
      "yu-scene".tag = "tracker-YS";
    };
    # Categories scanned for no-hardlink tagging. ignore_root_dir=false is REQUIRED
    # here: root_dir (/mnt/media/) contains the library (/mnt/media/library), so the
    # default (true) discounts the arr's library hardlinks as "internal" and flags
    # nearly everything noHL. false → any nlink>1 counts, so library-held torrents
    # are correctly NOT noHL. (A cross-seed sibling link also counts → cross-seeded
    # torrents still won't flag, as noted.)
    nohardlinks = lib.genAttrs (lib.attrNames categories) (_: {
      ignore_root_dir = false;
    });
  };
}
