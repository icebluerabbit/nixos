# Refresh Jellyfin's music library the moment a track is imported, rather than
# waiting for its scheduled scan — twelve hours away on this server's schedule.
#
# Lidarr's built-in MediaBrowser connection authenticates with the X-Emby-Token
# header and the api_key query parameter, both of which Jellyfin 12 removed, so
# its library update returns 401 and the import stays invisible. No released
# Lidarr carries the fix, so the refresh goes out through jellyfin-refresh: the
# same targeted /Library/Media/Updated over the Authorization: MediaBrowser
# scheme Jellyfin 12 accepts, rescanning only the artist's directory.
#
# The flags live in a wrapper because a custom script carries only `path` — the
# *arr's own Arguments field is gone in current releases. The wrapper is where
# this host varies the fixed binary's configuration.
#
# The key is the one thing not on argv, which /proc makes world-readable: it
# arrives as HOMELAB_JELLYFIN_REFRESH_API_KEY in lidarr-env and the hook
# inherits it from lidarr.service.
{
  config,
  lib,
  pkgs,
  ...
}:
let
  jellyfin = config.infra.clients.container-jellyfin;

  # The artist folder rather than the track file: a release import writes a whole
  # album directory, and Jellyfin groups tracks under the artist anyway.
  hook = pkgs.writeShellApplication {
    name = "jellyfin-refresh-lidarr";
    text = ''
      exec ${lib.getExe pkgs.homelab.jellyfin-refresh} \
        --base http://${jellyfin.ip}:${toString jellyfin.ports.http} \
        --event-env lidarr_eventtype \
        --path-env lidarr_artist_path "$@"
    '';
    meta.description = "Lidarr's jellyfin-refresh hook, with this host's flags";
  };
in
{
  name = "Jellyfin";
  implementation = "CustomScript";
  path = lib.getExe hook;
  on_release_import = true;
  on_upgrade = true;
  on_rename = true;
}
