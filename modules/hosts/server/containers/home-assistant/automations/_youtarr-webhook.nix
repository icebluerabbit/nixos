# youtarr's Apprise notifier → #youtarr. Apprise json:// POSTs
# {version, title, message, attachments, type} to youtarr's Vector sidecar, which
# republishes to MQTT. youtarr has no rich-format registry entry for json://, so
# every event (download run, auto-removal, test) arrives pre-rendered as plain
# text — nothing to branch on, so this is a straight relay.
_:
let
  target = [ "#youtarr:matrix.services.internal" ];
in
{
  id = "youtarr_webhook";
  alias = "Youtarr Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/youtarr";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      action = "matrix.send_message";
      data = {
        message = builtins.readFile ./msg/youtarr/Notify.tmpl;
        data.format = "html";
        inherit target;
      };
    }
  ];
  mode = "queued";
}
