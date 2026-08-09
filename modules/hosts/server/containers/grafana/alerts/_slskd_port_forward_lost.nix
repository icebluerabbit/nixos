# slskd's forwarded port is gone — the same failure as the qbt port-forward rule, for the
# Soulseek client. slskd holds its own broker claim + NAT-PMP lease; if it lapses slskd stays up
# and connected but can't accept incoming peer connections — fewer/slower uploads, worse queue
# position, and nothing says so. The broker exports vpngw_tunnel_forwarded_port{owner="slskd"}
# only while the lease is live, so its ABSENCE is the fault. Present → absent() empty → noData → OK.
{ promRule, ... }:
{
  group = "vpn-gw";
  rules = [
    (promRule {
      uid = "vpn_port_forward_lost_slskd";
      title = "slskd has no forwarded port";
      expr = ''absent(vpngw_tunnel_forwarded_port{owner="slskd"})'';
      op = "gt";
      value = 0;
      for = "10m";
      severity = "warning";
      category = "vpn";
      noDataState = "OK";
      summary = "slskd has no forwarded port";
      description = "No vpngw_tunnel_forwarded_port for slskd for 10m — its NAT-PMP lease from the Proton gateway lapsed or its claim was voided. slskd is up but can't accept incoming Soulseek connections.";
    })
  ];
}
