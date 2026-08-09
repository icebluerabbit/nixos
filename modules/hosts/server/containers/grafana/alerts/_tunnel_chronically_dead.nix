# One tunnel dead for hours. Replaces the old per-tunnel handshake alert that paged on every
# transient drop: this fires only when a SPECIFIC server has been unhealthy continuously for 2h —
# i.e. it isn't coming back on its own and its conf should be regenerated onto a fresh Proton
# server. Fires per tunnel; rare and actionable, not noise. Complements pool-capacity (aggregate)
# and flapping (churny-but-recovering).
# noDataState=OK: a series vanishes only when the broker stops reporting (container-down covers).
{ promRule, ... }:
{
  group = "vpn-gw";
  rules = [
    (promRule {
      uid = "vpn_tunnel_chronically_dead";
      title = "VPN tunnel dead >2h";
      expr = ''vpngw_tunnel_healthy{host="vpn-gw"}'';
      op = "lt";
      value = 1;
      for = "2h";
      severity = "warning";
      category = "vpn";
      noDataState = "OK";
      summary = "VPN tunnel {{ $labels.tunnel }} dead >2h";
      description = "{{ $labels.tunnel }} has been unhealthy for 2h straight — the broker isn't recovering it (Proton server likely gone). Regenerate its conf onto a fresh server (dedup on endpoint + peer pubkey, not the #label).";
    })
  ];
}
