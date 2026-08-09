# Soulseek notifications → #soulseek Matrix room. One topic (homelab/notify/soulseek) fed by
# two Vector producers: slskd (per-file download completed/failed) and soularr (grabbed /
# downloaded / import-failed / search-failed). Room is fixed; the choose picks the template
# per `.event`.
_:
let
  target = [ "#soulseek:matrix.services.internal" ];
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
  id = "soulseek_webhook";
  alias = "Soulseek Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/soulseek";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [
        # slskd — transfer layer (per file)
        (branch "download_completed" ./msg/soulseek/DownloadCompleted.tmpl)
        (branch "download_failed" ./msg/soulseek/DownloadFailed.tmpl)
        # soularr — album/orchestration layer
        (branch "grabbed" ./msg/soulseek/Grabbed.tmpl)
        (branch "downloaded" ./msg/soulseek/Downloaded.tmpl)
        (branch "import_failed" ./msg/soulseek/ImportFailed.tmpl)
        (branch "search_failed" ./msg/soulseek/SearchFailed.tmpl)
      ];
      default = [
        {
          action = "matrix.send_message";
          data = {
            message = "{{ trigger.payload_json | tojson }}";
            inherit target;
          };
        }
      ];
    }
  ];
  mode = "queued";
}
