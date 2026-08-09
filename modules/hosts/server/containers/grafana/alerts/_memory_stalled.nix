# The host is stalled reclaiming memory. Same PSI mechanism as io_stalled, different
# cause and different fix: this is time spent in direct reclaim and compaction — kswapd
# and kcompactd burning wall clock to find free pages — rather than time spent waiting
# for a device.
#
# Kept separate from io_stalled deliberately. They frequently fire together (reclaim
# pressure drives swap traffic, which is IO), but the remediation differs: memory
# pressure means something is oversized, IO pressure means something is too slow. One
# merged "resource pressure" alert would say neither.
#
# Threshold is far lower than the IO one. Memory stalls are pathological in a way IO
# stalls are not: a healthy host reclaims lazily in the background and registers ~0.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "memory_stalled";
      title = "Memory stalled";
      expr = ''rate(node_pressure_memory_stalled_seconds_total{host_type="host"}[5m])'';
      op = "gt";
      value = 0.05;
      lookback = 900;
      for = "15m";
      severity = "warning";
      category = "host";
      noDataState = "OK";
      keepFiringFor = "15m";
      summary = "{{ $labels.host }} is stalled reclaiming memory";
      description = "{{ $labels.host }} has spent more than 5% of the last 15m with every task blocked in memory reclaim. A process is holding more than the machine can serve — this is the state that precedes an OOM kill.";
    })
  ];
}
