# Refresh Jellyfin's movie library the moment a file is imported, rather than
# waiting for its scheduled scan — twelve hours away on this server's schedule.
#
# Radarr's built-in MediaBrowser connection authenticates with the X-Emby-Token
# header and the api_key query parameter, both of which Jellyfin 12 removed, so
# its library update returns 401 and the import stays invisible (Radarr#11658 —
# fixed upstream in 6.4.2+, this host runs 6.3.0, and no released Sonarr or
# Lidarr carries the fix at all). jellyfin-refresh posts the same targeted
# /Library/Media/Updated over the Authorization: MediaBrowser scheme Jellyfin 12
# accepts, so only the imported movie's directory is rescanned.
#
# The flags live in a wrapper because Radarr 6 rejects the notification's own
# Arguments field — "Arguments are no longer supported for custom scripts" — so
# `path` is the only thing it will carry. The wrapper is where this host varies
# the fixed binary's configuration, the way a systemd unit does elsewhere.
#
# The key is the one thing not on argv, which /proc makes world-readable: it
# arrives as HOMELAB_JELLYFIN_REFRESH_API_KEY in radarr-env and the hook
# inherits it from radarr.service.
{
  config,
  lib,
  pkgs,
  ...
}:
let
  jellyfin = config.infra.clients.container-jellyfin;

  # Radarr fills a different subset of the path variables per event, so both are
  # named and the first one carrying a path wins.
  hook = pkgs.writeShellApplication {
    name = "jellyfin-refresh-radarr";
    text = ''
      exec ${lib.getExe pkgs.homelab.jellyfin-refresh} \
        --base http://${jellyfin.ip}:${toString jellyfin.ports.http} \
        --event-env radarr_eventtype \
        --path-env radarr_movie_path,radarr_moviefile_path "$@"
    '';
    meta.description = "Radarr's jellyfin-refresh hook, with this host's flags";
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
