/*
  whipper — secure CD ripper (EAC successor): cdparanoia core, AccurateRip
  verification, MusicBrainz disc-ID metadata. Rips per-track FLAC + cue + log.

  whipper.conf is managed here (read-only). The drive section values came from a
  one-time detection against the ASUS SDRW-08U9M-U USB drive:
    read_offset    ← `whipper offset find`   (6)
    defeats_cache  ← `whipper drive analyze` (drive can defeat audio cache)
  To re-detect (new drive / changed offset), comment out xdg.configFile below so
  whipper can write, run the two commands, then re-bake the new values.

  `whippit` wraps the rip: rips into a throwaway /tmp/rip.XXXX staging dir (whipper
  builds the Artist/Album subdirs inside from MusicBrainz metadata), summarizes the
  AccurateRip results, saves that summary as whippit-summary.log in the album dir,
  then asks y/n to keep (merge-copy the album subdirs into /mnt/media/rips/ for the
  server's beets to import) or discard (rm -rf the staging dir).
  https://github.com/whipper-team/whipper
*/
_: {
  den.aspects.cli._.whipper = {
    aspect-name = "cli.whipper";
    homeManager =
      { pkgs, ... }:
      let
        whippit = pkgs.writeShellApplication {
          name = "whippit";
          runtimeInputs = [
            pkgs.whipper
            pkgs.coreutils
            pkgs.findutils
            pkgs.gnugrep
          ];
          text = ''
            tmp=$(mktemp -d /tmp/rip.XXXXXXXX)
            echo ">> ripping to $tmp ..."
            whipper cd rip -O "$tmp" || true

            mapfile -t logs < <(find "$tmp" -type f -name '*.log')
            if [ "''${#logs[@]}" -eq 0 ]; then
              echo "!! no .log in $tmp — rip produced nothing. discarding." >&2
              rm -rf "$tmp"
              exit 1
            fi

            albumdir=$(dirname "''${logs[0]}")
            flacs=$(find "$tmp" -type f -name '*.flac' | wc -l)
            ripped=0
            notver=0
            for f in "''${logs[@]}"; do
              ripped=$(( ripped + $(grep -ci 'accurately ripped' "$f" || true) ))
              notver=$(( notver + $(grep -ci 'cannot be verified' "$f" || true) ))
            done

            {
              echo "==== AccurateRip summary: $tmp ===="
              echo "FLAC tracks : $flacs"
              echo "AccurateRip : $ripped accurate, $notver not verified"
              echo
              echo "---- per-track ----"
              grep -hiE 'track [0-9]|accurately ripped|cannot be verified|status:|peak level' \
                "''${logs[@]}" || true
              echo "===================================="
            } | tee "$albumdir/whippit-summary.log"
            echo

            if [ "$notver" -gt 0 ]; then
              echo "!! $notver track(s) NOT AccurateRip-verified — rare disc or bad rip." >&2
            fi

            read -rp "Keep this rip for beets? [y/N] " ans
            case "$ans" in
              y | Y)
                mkdir -p /mnt/media/rips
                # cp -a (not mv): /tmp is tmpfs and /mnt/media is NFS, so this is
                # always a cross-device copy — and mv cannot merge into an existing
                # non-empty dir (whipper's Type dir, e.g. ep/, may already be there
                # from a prior rip), failing with "Directory not empty". cp -a merges
                # dir-by-dir; then drop the tmp staging.
                cp -a "$tmp"/. /mnt/media/rips/
                rm -rf "$tmp"
                echo ">> promoted album subdirs to /mnt/media/rips/ — beets will import"
                ;;
              *)
                rm -rf "$tmp"
                echo ">> discarded $tmp"
                ;;
            esac
          '';
        };
      in
      {
        home.packages = [
          pkgs.whipper
          whippit
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
