# managarr recorder — a SECOND Connect entry alongside _home-assistant.nix,
# not a replacement: that one feeds the Vector sidecar → MQTT → HA path.
#
# Lidarr names its events differently from Sonarr/Radarr — on_release_import
# rather than on_download, and no *_delete_for_upgrade at all. Recording both
# the import and the failure events is how phase two learns what a Lidarr
# supersede actually looks like on the wire.
{ config, ... }:
let
  m = config.infra.clients.container-managarr;
in
{
  name = "managarr";
  implementation = "Webhook";
  url = "http://${m.ip}:${toString m.ports.default}/hook/lidarr";
  method = 1; # POST
  on_grab = true;
  on_release_import = true;
  on_upgrade = true;
  on_rename = true;
  on_artist_add = true;
  on_artist_delete = true;
  on_album_delete = true;
  on_health_issue = true;
  on_health_restored = true;
  on_download_failure = true;
  on_import_failure = true;
  on_track_retag = true;
  on_application_update = true;
  include_health_warnings = true;
}
