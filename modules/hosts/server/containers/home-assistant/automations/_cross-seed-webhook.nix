# cross-seed's own notifications (result/inject) → #cross-seed. cross-seed POSTs
# {title, body, extra} to its notificationWebhookUrls; own webhook_id = sender identity.
# Distinct from the autobrr announce POST-ack (that WEBHOOK "accepted" is swallowed).
_:
let
  target = [ "#cross-seed:matrix.services.internal" ];
in
{
  id = "cross_seed_webhook";
  alias = "cross-seed Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/cross-seed";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      action = "matrix.send_message";
      data = {
        message = builtins.readFile ./msg/cross-seed/Result.tmpl;
        data.format = "html";
        inherit target;
      };
    }
  ];
  mode = "queued";
}
