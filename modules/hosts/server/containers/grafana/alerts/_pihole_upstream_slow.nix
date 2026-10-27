# Pi-hole sits in front of every public lookup the desktop and VPN clients make, so
# its upstream latency is felt directly as slow page loads. The desktop's dnsmasq
# caches for min-cache-ttl=120, which hides brief spikes — a sustained one has
# already outlived that buffer.
{ promRule, ... }:
{
  group = "network";
  rules = [
    (promRule {
      uid = "pihole_upstream_slow";
      title = "Pi-hole upstream slow";
      expr = "max(pihole_forward_destinations_responsetime{destination!~\"cache|blocklist\"})";
      op = "gt";
      value = 250;
      for = "15m";
      severity = "warning";
      category = "dns";
      noDataState = "OK";
      # Response time is inherently bursty; keep_firing_for stops a rule that is
      # genuinely firing from flapping resolved on a single quiet evaluation.
      keepFiringFor = "10m";
      summary = "Pi-hole upstream response time over 250ms";
      description = "An upstream resolver behind Pi-hole has averaged over 250ms for 15m. Public lookups from desktop and VPN clients are slow; internal names are unaffected.";
    })
  ];
}
