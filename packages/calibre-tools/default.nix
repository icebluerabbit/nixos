/*
  calibre-tools — library bootstrap and auto-ingest for a Calibre-Web container.
  https://calibre-ebook.com

    calibre-library-init  create an empty library if metadata.db is missing
    calibre-ingest        convert non-epub to epub, add to the library, delete
                          the source

  calibre is deliberately NOT in runtimeInputs: both scripts call calibredb and
  ebook-convert off PATH, and the caller supplies them via the unit's `path`.
  Pinning it here would drag the whole calibre closure into every consumer.
*/
{
  writeShellApplication,
  symlinkJoin,
  coreutils,
  findutils,
}:
let
  common = [
    coreutils
    findutils
  ];
in
symlinkJoin {
  name = "calibre-tools";
  paths = [
    (writeShellApplication {
      name = "calibre-library-init";
      runtimeInputs = common;
      text = builtins.readFile ./library-init.sh;
    })
    (writeShellApplication {
      name = "calibre-ingest";
      runtimeInputs = common;
      # nounset only — the original ran under `set -u` and relies on individual
      # file failures being skipped rather than aborting the batch.
      bashOptions = [ "nounset" ];
      text = builtins.readFile ./ingest.sh;
    })
  ];
  meta.description = "Calibre library bootstrap + auto-ingest helpers";
}
