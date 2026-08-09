# Sustained latency to the ISP. Bursty by nature (a saturated uplink spikes it), hence
# a long `for` plus keep_firing_for — the point is to catch a degraded line, not to
# narrate every torrent.
{ promRule, ... }:
{
  group = "network";
  rules = [
    (promRule {
      uid = "isp_latency";
      title = "ISP latency high";
      expr = "unpoller_site_latency_seconds";
      op = "gt";
      value = 0.1;
      for = "15m";
      severity = "warning";
      category = "unifi";
      noDataState = "OK";
      keepFiringFor = "10m";
      summary = "ISP latency above 100ms";
      description = "The gateway has measured >100ms to the ISP for 15m straight.";
    })
  ];
}
