# mousetrap notify → #myanonamouse. mousetrap POSTs {"type","payload"} to its Vector
# sidecar (loopback), republished to homelab/notify/mousetrap; HA consumes via an mqtt
# trigger. Branch on the top-level type; each event renders its own HTML card. No
# grab-approval loop like autobrr — mousetrap is the grab authority and already decides.
_:
let
  target = [ "#myanonamouse:matrix.services.internal" ];
  branch = evt: tmpl: {
    conditions = [
      {
        condition = "template";
        value_template = "{{ trigger.payload_json.type == '${evt}' }}";
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
  id = "mousetrap_webhook";
  alias = "mousetrap Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/mousetrap";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [
        (branch "announce" ./msg/mousetrap/Announce.tmpl)
        (branch "grab" ./msg/mousetrap/Grab.tmpl)
        (branch "enqueue" ./msg/mousetrap/Enqueue.tmpl)
        (branch "reject" ./msg/mousetrap/Reject.tmpl)
        (branch "hold" ./msg/mousetrap/Hold.tmpl)
        (branch "bonus_received" ./msg/mousetrap/BonusReceived.tmpl)
        (branch "spend" ./msg/mousetrap/Spend.tmpl)
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
