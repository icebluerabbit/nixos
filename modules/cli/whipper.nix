/*
  whipper — secure CD ripper (EAC successor): cdparanoia core, AccurateRip
  verification, MusicBrainz disc-ID metadata. Rips per-track FLAC + cue + log.

  whipper.conf is managed here (read-only). The drive section values came from a
  one-time detection against the ASUS SDRW-08U9M-U USB drive:
    read_offset    ← `whipper offset find`   (6)
    defeats_cache  ← `whipper drive analyze` (drive can defeat audio cache)
  To re-detect (new drive / changed offset), comment out xdg.configFile below so
  whipper can write, run the two commands, then re-bake the new values.

  `whippit` (pkgs.homelab.whippit, see packages/whippit) is the rip wrapper that
  drives whipper through a staging dir and a keep/discard prompt.
  https://github.com/whipper-team/whipper
*/
_: {
  den.aspects.cli._.whipper = {
    aspect-name = "cli.whipper";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [
          pkgs.whipper
          pkgs.homelab.whippit
        ];

        # Section header is URL-encoded (%20 space, %3A colon) exactly as whipper
        # emits it from the drive's ATAPI identity string — keep verbatim.
        xdg.configFile."whipper/whipper.conf".text = ''
          [main]
          path_filter_fat = True

          [drive:ASUS%20%20%20%20%3ASDRW-08U9M-U%20%20%20%20%3AA114]
          vendor = ASUS
          model = SDRW-08U9M-U
          release = A114
          read_offset = 6
          defeats_cache = True

          [whipper.cd.rip]
          unknown = True
        '';
      };
  };
}
