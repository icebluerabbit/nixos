# Home Assistant notifier → local Vector sidecar (loopback) → MQTT
# (homelab/notify/prowlarr); HA consumes via an mqtt trigger. No direct
# webhook to HA. The arr Webhook connector sends Content-Type: application/json.
_: {
  name = "Home Assistant";
  implementation = "Webhook";
  url = "http://127.0.0.1:9001/";
  method = 1; # POST
  on_grab = true;
  on_health_issue = true;
  on_health_restored = true;
  on_application_update = true;
  include_health_warnings = true;
}
