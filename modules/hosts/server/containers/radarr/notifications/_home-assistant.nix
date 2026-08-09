# Home Assistant notifier — fires on every event. Posts to the local Vector
# sidecar (loopback), which republishes to MQTT (homelab/notify/radarr); HA
# consumes via an mqtt trigger. No direct webhook to HA.
_: {
  name = "Home Assistant";
  implementation = "Webhook";
  url = "http://127.0.0.1:9001/";
  method = 1; # POST
  on_grab = true;
  on_download = true;
  on_upgrade = true;
  on_rename = true;
  on_movie_added = true;
  on_movie_delete = true;
  on_movie_file_delete = true;
  on_movie_file_delete_for_upgrade = true;
  on_health_issue = true;
  on_health_restored = true;
  on_application_update = true;
  on_manual_interaction_required = true;
  include_health_warnings = true;
}
