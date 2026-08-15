# Pi-hole is the desktop's only public forwarder (services.dns-cache lists it alone,
# deliberately — a second server would race it and leak ads), so FTL going down takes
# public DNS with it. *.internal keeps working via the dnsmasq container, which is why
# this is a warning and not a critical: the homelab stays administrable.
{ promRule, ... }:
{
  group = "network";
  rules = [
    (promRule {
      uid = "pihole_down";
      title = "Pi-hole down";
      expr = "pihole_status";
      op = "lt";
      value = 1;
      for = "5m";
      severity = "warning";
      category = "dns";
      # The exporter only publishes while it can reach FTL's API, so a vanished
      # series means the same thing a 0 does — the resolver is not answering.
      noDataState = "Alerting";
      missingSeriesEvalsToResolve = 3;
      summary = "Pi-hole is not resolving";
      description = "pihole_status has been 0 (or absent) for 5m. Public DNS on desktop and VPN clients is down; *.internal still resolves via the dnsmasq container.";
    })
  ];
}
