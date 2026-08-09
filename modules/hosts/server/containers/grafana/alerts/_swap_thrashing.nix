# Pages are being read back from swap. This is the metric that means something:
# swap_exhausted watches how full swap is, which on a file-serving host is mostly cold
# anon parked to make room for page cache and stays near-full while nothing suffers.
# Swap-in off zero means the working set genuinely exceeds RAM and the machine is
# paying disk latency for memory it needs.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "swap_thrashing";
      title = "Swap thrashing";
      expr = ''rate(node_vmstat_pswpin{host_type="host"}[5m]) * 4096'';
      op = "gt";
      value = 5242880; # 5 MB/s
      for = "15m";
      severity = "warning";
      category = "host";
      noDataState = "OK";
      summary = "{{ $labels.host }} is reading back from swap";
      description = "{{ $labels.host }} has been swapping pages back in at more than 5 MB/s for 15m. Unlike swap usage, this means memory the machine actually needs is on disk.";
    })
  ];
}
