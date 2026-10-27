/*
  seasonpackarr's notifications → #seasonpackarr.

  seasonpackarr only ships a Discord notifier, but it validates nothing about
  the URL, so it POSTs its embed to the container's own Vector sidecar instead
  (see hosts/server/containers/seasonpackarr/seasonpackarr.nix). The sidecar
  republishes the body verbatim to homelab/notify/seasonpackarr, so the payload
  here is a Discord message: {content, embeds: [{title, color, fields, …}]}.

  Branching is on the embed title, which is `BuildTitle(statusCode)` — upstream's
  StatusCode string with its first letter capitalised, so it is a closed enum
  from internal/domain/http.go and the exact set is reproduced below.

  (The embed also carries a colour picked from the same status, but three magic
  ints are a far worse thing to match on than the strings they were derived
  from: unreadable in a diff, and wrong silently rather than loudly.)

  `rejected` is enumerated rather than used as the fallback so that the default
  is the *loud* branch: a status added upstream shows up as an error card rather
  than being quietly filed as a routine miss. The 12 titles below are exactly
  NotificationStatusMap[INFO]; everything else upstream defines is one of the
  12 "could not …"/"error …" statuses in NotificationStatusMap[ERROR].

  Note that size/season/episode mismatches are in no bucket at all upstream, so
  they never notify — they appear only in the logs and in the
  seasonpackarr_mismatches_total metric.

  INFO is the chatty one: notificationLevel includes it so the rejection reasons
  reach the metrics, which means every announce failing smartMode posts here. To
  quieten it, set the rejected branch's `sequence = [ ]` — the metrics are
  derived in Vector before this automation runs, so they keep working.
*/
_:
let
  target = [ "#seasonpackarr:matrix.services.internal" ];
  send = tmpl: [
    {
      action = "matrix.send_message";
      data = {
        message = builtins.readFile tmpl;
        data.format = "html";
        inherit target;
      };
    }
  ];

  # NotificationStatusMap[INFO] — a pack that simply did not match. Written
  # lowercased because the comparison lowercases the incoming title (upstream's
  # "HDR metadata did not match" is the only one that differs from its source
  # string).
  rejectedTitles = [
    "no matching releases in client"
    "number of matches below threshold"
    "release already in client"
    "release is not a season pack"
    "resolution did not match"
    "source did not match"
    "release group did not match"
    "cut did not match"
    "edition did not match"
    "repack status did not match"
    "hdr metadata did not match"
    "streaming service did not match"
  ];

  # BuildTitle only capitalises the first letter, so lowercasing both sides
  # makes the comparison exact against the upstream strings.
  onTitle = titles: [
    {
      condition = "template";
      value_template =
        "{{ (trigger.payload_json.embeds[0].title | default('') | lower) in "
        + builtins.toJSON titles
        + " }}";
    }
  ];
in
{
  id = "seasonpackarr_webhook";
  alias = "seasonpackarr Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/seasonpackarr";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [
        {
          conditions = onTitle [ "successful match" ];
          sequence = send ./msg/seasonpackarr/Matched.tmpl;
        }
        {
          conditions = onTitle rejectedTitles;
          sequence = send ./msg/seasonpackarr/Rejected.tmpl;
        }
      ];
      # Every remaining upstream status is an error ("could not create
      # hardlinks", "could not get client", "error decoding request", …), as is
      # anything new upstream adds.
      default = send ./msg/seasonpackarr/Error.tmpl;
    }
  ];
  mode = "queued";
}
