# Sonarr webhook → #sonarr Matrix room. Own webhook_id = sender identity,
# so routing needs no instanceName check; branch only picks the event template.
_:
let
  target = [ "#sonarr:matrix.services.internal" ];
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
  id = "sonarr_webhook";
  alias = "Sonarr Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/sonarr";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [
        (branch "Test" ./msg/sonarr/Test.tmpl)
        (branch "Grab" ./msg/sonarr/Grab.tmpl)
        (branch "Download" ./msg/sonarr/Download.tmpl)
        (branch "SeriesAdd" ./msg/sonarr/SeriesAdd.tmpl)
        (branch "SeriesDelete" ./msg/sonarr/SeriesDelete.tmpl)
        (branch "EpisodeFileDelete" ./msg/sonarr/EpisodeFileDelete.tmpl)
        (branch "ManualInteractionRequired" ./msg/sonarr/ManualInteractionRequired.tmpl)
        (branch "Rename" ./msg/sonarr/Rename.tmpl)
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
        (branch "Health" ./msg/sonarr/Health.tmpl)
        (branch "HealthRestored" ./msg/sonarr/HealthRestored.tmpl)
        (branch "ApplicationUpdate" ./msg/sonarr/ApplicationUpdate.tmpl)
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
