# Refresh Jellyfin's TV library the moment a file is imported, rather than
# waiting for its scheduled scan — twelve hours away on this server's schedule.
#
# Sonarr's built-in MediaBrowser connection authenticates with the X-Emby-Token
# header and the api_key query parameter, both of which Jellyfin 12 removed, so
# its library update returns 401 and the import stays invisible. Sonarr fixed
# this upstream (commit be1dc03, "Fixed: Connecting to Jellyfin 12+") but no
# released 4.0.19 carries it, so the refresh goes out through jellyfin-refresh:
# the same targeted /Library/Media/Updated over the Authorization: MediaBrowser
# scheme Jellyfin 12 accepts, rescanning only the series' directory.
#
# The flags live in a wrapper because a custom script carries only `path` — the
# *arr's own Arguments field is gone in current releases. The wrapper is where
# this host varies the fixed binary's configuration.
#
# The key is the one thing not on argv, which /proc makes world-readable: it
# arrives as HOMELAB_JELLYFIN_REFRESH_API_KEY in sonarr-env and the hook
# inherits it from sonarr.service.
{
  config,
  lib,
  pkgs,
  ...
}:
let
  jellyfin = config.infra.clients.container-jellyfin;

  # The series folder is preferred over the episode file: an import writes
  # subtitles and artwork beside the file, and Jellyfin refreshes exactly the
  # path it is handed.
  hook = pkgs.writeShellApplication {
    name = "jellyfin-refresh-sonarr";
    text = ''
      exec ${lib.getExe pkgs.homelab.jellyfin-refresh} \
        --base http://${jellyfin.ip}:${toString jellyfin.ports.http} \
        --event-env sonarr_eventtype \
        --path-env sonarr_series_path,sonarr_episodefile_path "$@"
    '';
    meta.description = "Sonarr's jellyfin-refresh hook, with this host's flags";
  };
in
{
  name = "Jellyfin";
  implementation = "CustomScript";
  path = lib.getExe hook;
  on_download = true;
  on_upgrade = true;
  on_rename = true;
}
