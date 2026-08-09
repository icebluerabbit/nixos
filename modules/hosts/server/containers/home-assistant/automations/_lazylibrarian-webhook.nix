# LazyLibrarian custom-notifier → #lazylibrarian Matrix room. LL has no webhook of
# its own; its notify script POSTs the book's DB row to the Vector sidecar, which
# republishes to homelab/notify/lazylibrarian. Branch on Event — LL only ever emits
# these four (scheduling.py: notify_strings), so there is no eventType-style field.
_:
let
  target = [ "#lazylibrarian:matrix.services.internal" ];
  branch = event: tmpl: {
    conditions = [
      {
        condition = "template";
        value_template = "{{ trigger.payload_json.Event == '${event}' }}";
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
  id = "lazylibrarian_webhook";
  alias = "LazyLibrarian Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/lazylibrarian";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [
        (branch "Started Download" ./msg/lazylibrarian/StartedDownload.tmpl)
        (branch "Added to Library" ./msg/lazylibrarian/AddedToLibrary.tmpl)
        (branch "Download failed" ./msg/lazylibrarian/DownloadFailed.tmpl)
        (branch "Test" ./msg/lazylibrarian/Test.tmpl)
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
