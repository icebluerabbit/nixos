# Grafana's own rule evaluation is failing. Every other rule in this directory is worth
# exactly as much as the evaluator behind it: a datasource that stops answering, a query
# that starts erroring, a rule group that can't schedule, and the fleet goes quiet in a
# way that is indistinguishable from healthy.
#
# This is the inner half of the watchdog problem and it only covers failures Grafana is
# alive to report. The outer half — Grafana itself down, the container gone, the host
# gone — cannot be solved from inside this stack at all, and wants an external check
# (a Healthchecks.io ping from a systemd timer, or anything else off this machine).
#
# Critical, so it short-circuits to #alerts rather than filing itself under telemetry
# history: if this fires, nothing else in here can be trusted to fire.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "alerting_broken";
      title = "Alert evaluation failing";
      expr = "increase(grafana_alerting_rule_evaluation_attempt_failures_total[15m])";
      op = "gt";
      value = 0;
      lookback = 1800;
      for = "10m";
      severity = "critical";
      category = "telemetry";
      noDataState = "OK";
      keepFiringFor = "30m";
      summary = "Grafana alert rules are failing to evaluate";
      description = "Grafana has failed to evaluate alert rules in the last 15m — a datasource is not answering or a rule query is erroring. Any alert that depends on the failing rule group is silent right now, and silence currently means nothing.";
    })
  ];
}
