# Bindery notify → #bindery. Bindery POSTs its notification payload to its Vector
# sidecar (loopback), republished to homelab/notify/bindery; HA consumes via an mqtt
# trigger. Unlike mousetrap's {"type","payload"} envelope, Bindery's payload is FLAT
# and carries `eventType` on every event (normalizeEventPayload), so branches key off
# the top-level field directly.
_:
let
  target = [ "#bindery:matrix.services.internal" ];
  branch = evt: tmpl: {
    conditions = [
      {
        condition = "template";
        value_template = "{{ trigger.payload_json.eventType == '${evt}' }}";
      }
    ];
    sequence = [
      {
        action = "matrix.send_message";
        data = {
          message = builtins.readFile tmpl;
          data.format = "html";
          inherit target;
        };
      }
    ];
  };
in
{
  id = "bindery_webhook";
  alias = "Bindery Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/bindery";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [
        (branch "grabbed" ./msg/bindery/Grabbed.tmpl)
        (branch "bookImported" ./msg/bindery/BookImported.tmpl)
        (branch "upgrade" ./msg/bindery/Upgrade.tmpl)
        (branch "downloadFailed" ./msg/bindery/DownloadFailed.tmpl)
        (branch "health" ./msg/bindery/Health.tmpl)
      ];
      default = [
        {
          action = "matrix.send_message";
          data = {
            message = "{{ trigger.payload_json | tojson(indent=2) }}";
            inherit target;
          };
        }
      ];
    }
  ];
  mode = "queued";
}
