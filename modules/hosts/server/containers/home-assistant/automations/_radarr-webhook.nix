# Radarr webhook → #radarr Matrix room. Own webhook_id = sender identity,
# so routing needs no instanceName check; branch only picks the event template.
_:
let
  target = [ "#radarr:matrix.services.internal" ];
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
  id = "radarr_webhook";
  alias = "Radarr Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/radarr";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [
        (branch "Test" ./msg/radarr/Test.tmpl)
        (branch "Grab" ./msg/radarr/Grab.tmpl)
        (branch "Download" ./msg/radarr/Download.tmpl)
        (branch "MovieAdded" ./msg/radarr/MovieAdded.tmpl)
        (branch "MovieDelete" ./msg/radarr/MovieDelete.tmpl)
        (branch "MovieFileDelete" ./msg/radarr/MovieFileDelete.tmpl)
        (branch "ManualInteractionRequired" ./msg/radarr/ManualInteractionRequired.tmpl)
        (branch "Rename" ./msg/radarr/Rename.tmpl)
        # UpdateCheck health warnings are "update available" noise that repeats
        # forever — swallow them. Real health issues still render below.
        {
          conditions = [
            {
              condition = "template";
              value_template = "{{ trigger.payload_json.eventType == 'Health' and trigger.payload_json.type == 'UpdateCheck' }}";
            }
          ];
          sequence = [ ];
        }
        (branch "Health" ./msg/radarr/Health.tmpl)
        (branch "HealthRestored" ./msg/radarr/HealthRestored.tmpl)
        (branch "ApplicationUpdate" ./msg/radarr/ApplicationUpdate.tmpl)
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
