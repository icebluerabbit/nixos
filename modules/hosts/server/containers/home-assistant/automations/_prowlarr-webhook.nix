# Prowlarr webhook → #prowlarr Matrix room. Own webhook_id = sender identity;
# branch only picks the event template.
_:
let
  target = [ "#prowlarr:matrix.services.internal" ];
  branch = event: tmpl: {
    conditions = [
      {
        condition = "template";
        value_template = "{{ trigger.payload_json.eventType == '${event}' }}";
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
  id = "prowlarr_webhook";
  alias = "Prowlarr Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/prowlarr";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [
        (branch "Test" ./msg/prowlarr/Test.tmpl)
        (branch "Grab" ./msg/prowlarr/Grab.tmpl)
        (branch "Health" ./msg/prowlarr/Health.tmpl)
        (branch "HealthRestored" ./msg/prowlarr/HealthRestored.tmpl)
        (branch "ApplicationUpdate" ./msg/prowlarr/ApplicationUpdate.tmpl)
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
