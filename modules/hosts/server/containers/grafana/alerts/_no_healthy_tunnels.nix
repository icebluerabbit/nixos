# All VPN tunnels unhealthy at once — the broker has nothing to hand out, so qbt loses its
# egress entirely (the killswitch drops torrent traffic). Distinct from a single tunnel_down:
# this is EVERY tunnel dead (Proton-wide outage / all nodes gone) — the pool-exhausted case.
# Broker health = exit-IP probe + natpmp lease, so it catches route/exit breakage a live wg
# handshake hides. noDataState=OK: no series means the broker/vpn-gw is dark, which is the
# target-down / container-down rule's job, not this one's.
{ promRule, ... }:
{
  group = "vpn-gw";
  rules = [
    (promRule {
      uid = "vpn_no_healthy_tunnels";
      title = "No healthy VPN tunnels";
      # sum over the 0/1 health gauges: reads 0 while all present tunnels are unhealthy (a
      # real value — the series still exist at 0), and empty only when the broker stops
      # reporting (→ noData → OK).
      expr = "sum(vpngw_tunnel_healthy)";
      op = "lt";
      value = 1;
      for = "3m";
      severity = "critical";
      category = "vpn";
      noDataState = "OK";
      summary = "All VPN tunnels unhealthy";
      description = "The vpn-gw broker reports zero healthy tunnels for 3m — no tunnel can be claimed, so qBittorrent has no VPN egress (killswitch drops all torrent traffic). Likely a Proton-wide outage.";
    })
  ];
}
