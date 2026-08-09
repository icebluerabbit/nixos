# qBittorrent's forwarded port is gone. The NAT-PMP lease from the Proton gateway is
# renewed continuously; when it lapses, qBittorrent stays up, stays connected, and
# quietly becomes unconnectable — no incoming peers, no seeding, and nothing anywhere
# says so. Complements the tunnel-down rule: the tunnel can be perfectly healthy while
# the forward is dead.
{ promRule, ... }:
{
  group = "vpn-gw";
  rules = [
    (promRule {
      uid = "vpn_port_forward_lost";
      title = "VPN port forward lost";
      # The broker exports vpngw_tunnel_forwarded_port only while qbt holds a claim WITH a
      # live lease. If the lease lapses (or the claim is voided) the series disappears, so we
      # alert on its ABSENCE. Present → absent() yields no data → noData → OK.
      expr = ''absent(vpngw_tunnel_forwarded_port{owner="qbittorrent"})'';
      op = "gt";
      value = 0;
      for = "10m";
      severity = "warning";
      category = "vpn";
      noDataState = "OK";
      summary = "qBittorrent has no forwarded port";
      description = "No vpngw_tunnel_forwarded_port for qbittorrent — the broker's NAT-PMP lease from the Proton gateway lapsed or the claim was voided. qBittorrent is up but unconnectable: no incoming peers.";
    })
  ];
}
