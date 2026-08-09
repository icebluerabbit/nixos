# autobrr WEBHOOK notification → local Vector sidecar (loopback) → MQTT
# (homelab/notify/autobrr); HA consumes via an mqtt trigger. No direct webhook.
# Fires on push events (approved/rejected/error) with autobrr's structured
# payload; the top-level `event` field carries the accept/reject state and
# `data.result.rejections` the reasons. autobrr sets Content-Type:
# application/json itself, so HA's trigger.payload_json populates.
_: {
  name = "Home Assistant";
  notification_type = "WEBHOOK";
  enabled = true;
  events = [
    "PUSH_APPROVED"
    "PUSH_REJECTED"
    "PUSH_ERROR"
  ];
  webhook = "http://127.0.0.1:9001/";
  method = "POST";
}
