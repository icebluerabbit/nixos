# Host / NixOS lifecycle events → #nixos Matrix room. Own webhook_id = sender
# identity; branch on the `event` field set by the host-side nixos-notify helper.
_:
let
  target = [ "#nixos:matrix.services.internal" ];
  branch = event: tmpl: {
    conditions = [
      {
        condition = "template";
        value_template = "{{ trigger.payload_json.event == '${event}' }}";
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
  id = "nixos_webhook";
  alias = "NixOS Host Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/nixos";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [
        (branch "boot" ./msg/nixos/Boot.tmpl)
        (branch "switch" ./msg/nixos/Switch.tmpl)
        (branch "scrub" ./msg/nixos/Scrub.tmpl)
        (branch "gc" ./msg/nixos/Gc.tmpl)
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
