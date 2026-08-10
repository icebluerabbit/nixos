/*
  soularr-reclaim — hand stranded Soulseek album folders back to Lidarr.

  A soularr run only imports what it grabbed in that same run, so albums
  stranded by a crash or an NFS stall sit in the download dir forever. This
  walks that dir and fires DownloadedAlbumsScan per album folder. Read-only
  with respect to the filesystem — Lidarr does any moving.

  Fully configured through the environment (LIDARR_URL, LIDARR_KEY_FILE,
  DOWNLOAD_DIR, RECLAIM_*), so it takes no arguments.
*/
{ writers }:
writers.writePython3Bin "soularr-reclaim" { } (builtins.readFile ./soularr_reclaim.py)
