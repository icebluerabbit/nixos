/*
  beets-lidarr-import — hand one finished album from beets to Lidarr.

  beets tags rips and writes them into an output directory; this asks Lidarr
  what it identifies there and then tells it to import exactly that, via the
  ManualImport command. Lidarr stays the only writer into the library, so rips
  and downloads are named and laid out identically and cannot drift apart.

  It does not use DownloadedAlbumsScan. That path scores the files against the
  releases Lidarr holds and refuses anything under 80%, with the release MBID
  weighted 5.0 of roughly 23, so a rip tagged with a real MusicBrainz release
  Lidarr's metadata mirror does not carry is rejected however correct it is.
  ManualImport builds its decisions directly and never runs that check.

  Called from beets' `hook` plugin on album_imported. Configuration arrives by
  environment (LIDARR_URL, LIDARR_API_KEY) and the album path by argv.

  Tests: nix build .#beets-lidarr-import.tests
         python -m unittest discover -p "*_test.py"   (from this directory)
*/
{
  writers,
  runCommand,
  python3,
}:
let
  program = writers.writePython3Bin "beets-lidarr-import" { } (
    builtins.readFile ./beets_lidarr_import.py
  );
in
program.overrideAttrs (old: {
  passthru = (old.passthru or { }) // {
    tests = runCommand "beets-lidarr-import-tests" { } ''
      # Without pipefail the build would pass on a failing test: tee exits
      # zero and produces $out regardless of what unittest returned.
      set -o pipefail
      cp ${./beets_lidarr_import.py} beets_lidarr_import.py
      cp ${./beets_lidarr_import_test.py} beets_lidarr_import_test.py
      # unittest only discovers test*.py by default; this repo names tests
      # after the file they cover, the way its Go packages do.
      ${python3}/bin/python -m unittest discover -p '*_test.py' \
        --verbose 2>&1 | tee $out
    '';
  };
})
