# Pi-hole lifecycle events → #dns Matrix room. The pi-hole Vector sidecar filters its
# journal down to four transitions and publishes each to
# homelab/notify/dns/events/<kind>; one wildcard subscription, branch on `kind`.
#
# Deliberately not alerts: these are things that happened, not things that are wrong.
# Anything that needs a threshold lives in grafana/alerts/_pihole_*.nix and reaches
# this same room by category, so the two paths never duplicate each other.
#
# The gravity message quotes the current blocklist size, which the event payload does
# not carry — the refresh log says it finished, not what it built. That number is read
# from Prometheus when the message is composed.
_:
let
  target = [ "#dns:matrix.services.internal" ];

  send = tmpl: {
    action = "matrix.send_message";
    data = {
      message = builtins.readFile tmpl;
      data.format = "html";
      inherit target;
    };
  };

  branch = kind: sequence: {
    conditions = [
      {
        condition = "template";
        value_template = "{{ trigger.payload_json.kind == '${kind}' }}";
      }
    ];
    inherit sequence;
  };
in
{
  id = "dns_events_webhook";
  alias = "DNS Events Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/dns/events/+";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [
        (branch "gravity_refreshed" [
          {
            action = "rest_command.prometheus_query";
            data.query = "pihole_domains_being_blocked";
            response_variable = "gravity";
          }
          (send ./msg/dns/GravityRefreshed.tmpl)
        ])
        (branch "gravity_failed" [ (send ./msg/dns/GravityFailed.tmpl) ])
        (branch "ftl_failed" [ (send ./msg/dns/FtlFailed.tmpl) ])
        (branch "ftl_restarted" [ (send ./msg/dns/FtlRestarted.tmpl) ])
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
