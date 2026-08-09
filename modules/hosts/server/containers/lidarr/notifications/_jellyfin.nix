# Refresh Jellyfin's music library the moment a track is imported, rather than
# waiting for its scheduled/inotify scan.
{ config, ... }:
{
  name = "Jellyfin";
  implementation = "MediaBrowser";
  host = config.infra.clients.container-jellyfin.ip;
  port = config.infra.clients.container-jellyfin.ports.http;
  api_key = "\${env.JELLYFIN_API_KEY}";
  update_library = true;
  notify = false;
  on_release_import = true;
  on_upgrade = true;
  on_rename = true;
}
