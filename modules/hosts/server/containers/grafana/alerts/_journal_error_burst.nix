# A container is spewing errors. PRIORITY <= 3 is journald's err/crit/alert/emerg — the
# levels a service uses when it thinks something is actually wrong, as opposed to the
# noisy `detected_level="error"` heuristic Loki infers from log text (~630/hr fleet-wide,
# mostly harmless). Baseline for PRIORITY <= 3 is a handful an hour, so 20 in five
# minutes means something started failing in a loop.
#
# Filters on the parsed PRIORITY field rather than a text match, which also keeps it from
# matching Grafana's own logs about running this query — see _oom_kill.nix.
{ lokiRule, ... }:
{
  group = "infra";
  rules = [
    (lokiRule {
      uid = "journal_error_burst";
      title = "Journal error burst";
      expr = ''sum by (host) (count_over_time({host_type="container"} | json | PRIORITY <= 3 [5m]))'';
      op = "gt";
      value = 20;
      for = "10m";
      severity = "warning";
      category = "host";
      noDataState = "OK";
      keepFiringFor = "10m";
      summary = "{{ $labels.host }} is logging errors in a loop";
      description = "{{ $labels.host }} has logged more than 20 err-or-worse journal entries in 5 minutes. Baseline is a handful an hour — something is failing repeatedly.";
    })
  ];
}
