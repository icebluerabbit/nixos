# Home Assistant notifier → local Vector sidecar (loopback) → MQTT
# (homelab/notify/lidarr); HA consumes via an mqtt trigger. No direct webhook.
_: {
  name = "Home Assistant";
  implementation = "Webhook";
  url = "http://127.0.0.1:9001/";
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
