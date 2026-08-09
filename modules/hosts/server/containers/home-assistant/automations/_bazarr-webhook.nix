# Bazarr webhook → #bazarr Matrix room. Wired declaratively via configuratarr's
# bazarr "JSON" Apprise provider (see bazarr/notifications/_home-assistant.nix),
# which POSTs {version,title,message,type} as application/json to this webhook.
_: {
  id = "bazarr_webhook";
  alias = "Bazarr Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/bazarr";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      action = "matrix.send_message";
      data = {
        message = "{% set t = trigger.payload_json.type | default('info') %}{% if t == 'failure' %}⚠️{% elif t == 'warning' %}🟡{% elif t == 'success' %}✅{% else %}💬{% endif %} <b>{{ trigger.payload_json.title | default('Bazarr') }}</b><br>{{ trigger.payload_json.message }}";
        data.format = "html";
        target = [ "#bazarr:matrix.services.internal" ];
      };
    }
  ];
  mode = "queued";
}
