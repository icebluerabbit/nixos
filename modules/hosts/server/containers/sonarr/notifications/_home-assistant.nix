# Home Assistant notifier — fires on every event. Posts to the local Vector
# sidecar (loopback), which republishes to MQTT (homelab/notify/sonarr); HA
# consumes via an mqtt trigger. No direct webhook to HA.
_: {
  name = "Home Assistant";
  implementation = "Webhook";
  url = "http://127.0.0.1:9001/";
  method = 1; # POST
  on_grab = true;
  on_download = true;
  on_upgrade = true;
  # Disabled: on_import_complete also emits eventType "Download" but with an
  # episodeFiles array (no singular episodeFile), so it renders an empty dup of
  # the per-file on_download card. Keep on_download only.
  on_import_complete = false;
  on_rename = true;
  on_series_add = true;
  on_series_delete = true;
  on_episode_file_delete = true;
  on_episode_file_delete_for_upgrade = true;
  on_health_issue = true;
  on_health_restored = true;
  on_application_update = true;
  on_manual_interaction_required = true;
  include_health_warnings = true;
}
