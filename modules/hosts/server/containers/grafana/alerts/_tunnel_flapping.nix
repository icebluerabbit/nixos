# A tunnel that keeps flapping — up, down, up, down — without ever staying dead long enough to
# trip the chronically-dead rule. The wg1/wg3/wg4 pattern: an unstable Proton server / congested
# city. flap_score = down-transitions in the broker's rolling 24h window; op gt 7 = fired 8+ times,
# so the broker is repeatedly re-homing clients off it (each re-home = brief interruption + new
# exit IP/port). Regenerate onto a stable city. The score is in-memory and resets on a deploy or
# broker restart, so it counts flaps since last restart — a lower bound, never an overcount.
# noDataState=OK: no series = broker dark = container-down's job.
{ promRule, ... }:
{
  group = "vpn-gw";
  rules = [
    (promRule {
      uid = "vpn_tunnel_flapping";
      title = "VPN tunnel flapping";
      expr = ''vpngw_tunnel_flap_score{host="vpn-gw"}'';
      op = "gt";
      value = 7;
      for = "15m";
      severity = "warning";
      category = "vpn";
      noDataState = "OK";
      summary = "VPN tunnel {{ $labels.tunnel }} flapping (8+ drops/24h)";
      description = "{{ $labels.tunnel }} has flapped 8+ times in the rolling 24h window — an unstable Proton server. The broker de-prioritizes it at claim time, but it's still churning clients. Regenerate its conf onto a stable city.";
    })
  ];
}
