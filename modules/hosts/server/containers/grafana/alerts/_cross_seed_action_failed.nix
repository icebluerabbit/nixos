# autobrr matched a release and then failed to hand it to cross-seed. autobrr_irc_down
# does not cover this and never could: the IRC connection is healthy, announces are
# arriving, filters are matching — the action at the end of the pipeline is what fails,
# so from every metric autobrr exports it is working perfectly while every grab is lost.
#
# The symptom in practice is a cross-seed that is up but not answering, e.g. blocked on
# IO behind a saturated disk or a hung NFS mount, which turns the webhook POST into a
# client timeout. autobrr logs one line per dropped release and carries on.
#
# Matching on the parsed `message` field rather than a bare line filter, per the loop
# documented in oom_kill: Grafana logs each alert query it runs, Vector ships that to
# Loki, and a raw substring match would find its own query text under host="grafana".
# The host="autobrr" selector already rules that out — the json parse keeps it true if
# this query is ever widened.
{ lokiRule, ... }:
{
  group = "media";
  rules = [
    (lokiRule {
      uid = "cross_seed_action_failed";
      title = "cross-seed action failing";
      expr = ''sum(count_over_time({host="autobrr"} | json | message =~ `.*process action failed.*` [15m]))'';
      op = "gt";
      value = 5;
      lookback = 1800;
      for = "0m";
      severity = "warning";
      category = "cross-seed";
      noDataState = "OK";
      keepFiringFor = "30m";
      summary = "autobrr cannot hand releases to cross-seed";
      description = "autobrr has failed more than 5 cross-seed actions in the last 15m. Releases are matching and then being dropped — cross-seed is up but not answering its webhook, usually because it is blocked on IO.";
    })
  ];
}
