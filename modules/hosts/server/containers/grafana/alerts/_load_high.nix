# Run queue far longer than the machine has cores. Normalised per host, because a raw
# load number means nothing on its own: 16 is idle on the 32-core server and a crisis on
# a 4-core box. count(node_cpu_seconds_total{mode="idle"}) is the core count.
#
# load15 rather than load1: the short averages spike on every rebuild, every scrub, every
# large import, and none of those are worth saying. Sustained load15 above 2x cores is a
# machine that is not keeping up.
#
# This is the coarse companion to io_stalled/memory_stalled — it fires on "too much work"
# regardless of what kind, where PSI fires on "not working at all". Keeping both means a
# CPU-bound overload (which registers no PSI) still gets caught.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "load_high";
      title = "Load high";
      expr = ''
        node_load15{host_type="host"}
          / on (host) group_left
        count by (host) (node_cpu_seconds_total{mode="idle"})
      '';
      op = "gt";
      value = 2;
      for = "30m";
      severity = "warning";
      category = "host";
      noDataState = "OK";
      summary = "{{ $labels.host }} load is {{ $value }}x its core count";
      description = "The 15-minute run queue on {{ $labels.host }} has been more than twice its core count for 30m. Everything scheduled on this host is queueing behind something.";
    })
  ];
}
