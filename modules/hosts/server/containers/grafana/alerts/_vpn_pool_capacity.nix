# Pool capacity warning — fewer than HALF the tunnels healthy. Individual tunnels dropping is
# expected now (the broker fails clients over onto healthy ones), so a per-tunnel down alert is
# noise; what matters is headroom. Below half the pool, healthy spares run thin and clients start
# contending for the few good tunnels — the case where slskd got starved while qbt held the only
# stable one. A ratio, not a fixed count, so it tracks the pool as tunnels are added/removed.
# noDataState=OK: no series = broker/vpn-gw dark = the container-down rule's job, not this one's.
{ promRule, ... }:
{
  group = "vpn-gw";
  rules = [
    (promRule {
      uid = "vpn_pool_capacity_low";
      title = "VPN pool capacity low";
      expr = "sum(vpngw_tunnel_healthy) / count(vpngw_tunnel_healthy)";
      op = "lt";
      value = 0.5;
      for = "10m";
      severity = "warning";
      category = "vpn";
      noDataState = "OK";
      summary = "Fewer than half the VPN tunnels healthy";
      description = "Under half the vpn-gw tunnel pool has been healthy for 10m. Failover headroom is gone — clients start contending for the few good tunnels (how slskd got starved while qbt held the only stable one). Check Proton status and regenerate the dead confs.";
    })
  ];
}
