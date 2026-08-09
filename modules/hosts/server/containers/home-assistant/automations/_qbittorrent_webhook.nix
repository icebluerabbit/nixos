# qBittorrent AutoRun hooks (added/finished) → #qbittorrent Matrix room.
# qbt's on-added/on-finished wrappers POST {event,name,category,size} here;
# branch on event to pick the message.
_:
let
  target = [ "#qbittorrent:matrix.services.internal" ];
  branch = event: message: {
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
          inherit message target;
          data.format = "html";
        };
      }
    ];
  };
in
{
  id = "qbittorrent_webhook";
  alias = "qBittorrent Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/qbittorrent";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [
        (branch "added" "➕ <b>Added</b>: {{ trigger.payload_json.name }} <i>({{ trigger.payload_json.category }})</i>")
        (branch "finished" "🏁 <b>Finished</b>: {{ trigger.payload_json.name }} <i>({{ trigger.payload_json.category }}, {{ trigger.payload_json.size | int | filesizeformat }})</i>")
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
