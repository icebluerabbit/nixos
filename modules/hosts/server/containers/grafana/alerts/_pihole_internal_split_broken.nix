/*
  The routing design rests on Pi-hole forwarding every *.internal domain to the
  dnsmasq container. That override is load-bearing and fragile: FTL hardcodes
  `local=/internal/` (ICANN reserved the TLD), and the per-VLAN `server=` lines only
  win because dnsmasq resolves by longest domain match. An FTL upgrade that changes
  that precedence would silently NXDOMAIN every internal name for VPN clients, who
  have no local split-forwarder to fall back on.

  Traffic to the internal resolver dropping to zero while Pi-hole is otherwise
  serving queries is what that regression looks like from the outside.
*/
{ promRule, ... }:
{
  group = "network";
  rules = [
    (promRule {
      uid = "pihole_internal_split_broken";
      title = "Pi-hole internal forwarding stopped";
      expr = "sum(pihole_forward_destinations{destination=~\"10\\\\..*\"})";
      op = "lt";
      value = 1;
      for = "30m";
      severity = "warning";
      category = "dns";
      # Empty vector = no internal destination reported at all, which is precisely
      # the failure. Silence here is the signal, not the absence of one.
      noDataState = "Alerting";
      summary = "Pi-hole is no longer forwarding to the internal resolver";
      description = "No queries have reached the dnsmasq container via Pi-hole for 30m. The local=/internal/ override may have regressed, which would NXDOMAIN internal names for VPN clients.";
    })
  ];
}
