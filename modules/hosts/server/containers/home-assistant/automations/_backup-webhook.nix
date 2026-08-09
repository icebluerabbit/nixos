# Backup job results → #backups Matrix room. Host-side restic service publishes
# {event:"backup", result:"success|failed", ...} to homelab/notify/backups; branch
# on the `result` field.
_:
let
  target = [ "#backups:matrix.services.internal" ];
  branch = result: tmpl: {
    conditions = [
      {
        condition = "template";
        value_template = "{{ trigger.payload_json.result == '${result}' }}";
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
  id = "backup_webhook";
  alias = "Backup Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/backups";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [
        (branch "success" ./msg/backups/Success.tmpl)
        (branch "failed" ./msg/backups/Failed.tmpl)
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
