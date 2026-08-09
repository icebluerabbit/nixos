# The kernel OOM-killer fired somewhere in the fleet. Journald is the only place this
# is visible: the victim process just disappears, and the service that owned it either
# restarts silently or degrades in a way no service-level metric explains.
#
# Loki rules must be instant metric queries, never log queries — a log line carries no
# value for the threshold to evaluate.
#
# The SYSLOG_IDENTIFIER="kernel" filter is not decoration, it is what stops this rule
# from alerting on itself. Grafana logs every alert query it runs at level=info, and the
# log line quotes the query verbatim — so a bare `|= "Out of memory: Killed"` matches
# Grafana's own journal describing this very query, which Vector then ships to Loki, and
# the rule fires on host="grafana" forever. Anchoring on a structured journald field
# instead of a raw substring breaks the loop: Grafana's stdout is never _TRANSPORT=kernel.
#
# Any log-based rule added here has the same exposure. Match a field, not a string.
{ lokiRule, ... }:
{
  group = "infra";
  rules = [
    (lokiRule {
      uid = "oom_kill";
      title = "OOM kill";
      expr = ''sum by (host) (count_over_time({host_type=~".+"} | json | SYSLOG_IDENTIFIER="kernel" | message =~ `Out of memory: Killed.*` [5m]))'';
      op = "gt";
      value = 0;
      for = "0m";
      severity = "critical";
      category = "host";
      noDataState = "OK";
      summary = "OOM killer fired on {{ $labels.host }}";
      description = "The kernel killed a process on {{ $labels.host }} to reclaim memory. Whatever it killed is gone without a service-level error.";
    })
  ];
}
