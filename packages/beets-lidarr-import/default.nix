/*
  beets-lidarr-import — hand one finished album from beets to Lidarr.

  beets tags rips and writes them into an output directory; this asks Lidarr to
  import that directory via its DownloadedAlbumsScan command, which is the same
  path a completed download takes. Lidarr stays the only writer into the
  library, so rips and downloads are named and laid out identically and cannot
  drift apart.

  Called from beets' `hook` plugin on album_imported. Configuration arrives by
  environment (LIDARR_URL, LIDARR_API_KEY) and the album path by argv.
*/
{
  writeShellApplication,
  curl,
  jq,
}:
writeShellApplication {
  name = "beets-lidarr-import";
  runtimeInputs = [
    curl
    jq
  ];
  text = builtins.readFile ./beets-lidarr-import.sh;
  meta.description = "Ask Lidarr to import a finished beets album via DownloadedAlbumsScan";
}
