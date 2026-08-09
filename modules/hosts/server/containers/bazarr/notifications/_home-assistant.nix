# Apprise "JSON" provider → local Vector sidecar (loopback) → MQTT
# (homelab/notify/bazarr); HA consumes via an mqtt trigger. json:// = plain HTTP
# POST of {version,title,message,type} as application/json. Sparse by name: only
# "JSON" is managed, bazarr's other providers stay untouched.
_: {
  name = "JSON";
  enabled = true;
  url = "json://127.0.0.1:9001/";
}
