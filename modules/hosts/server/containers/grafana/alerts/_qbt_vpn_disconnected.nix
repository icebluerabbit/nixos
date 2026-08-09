# qBittorrent's own view of its connection. Distinct from the tunnel being down: this
# is the client saying it cannot reach the swarm, which is what actually stops traffic.
{ promRule, ... }:
{
  group = "media";
  rules = [
    (promRule {
      uid = "qbt_vpn_disconnected";
      title = "qBittorrent disconnected";
      expr = ''qbittorrent_transfer_connection_status{connection_status="disconnected"}'';
      op = "gt";
      value = 0;
      for = "10m";
      severity = "warning";
      category = "qbittorrent";
      noDataState = "OK";
      summary = "qBittorrent reports disconnected";
      description = "qBittorrent has reported connection_status=disconnected for 10m — it cannot reach the swarm.";
    })
  ];
}
