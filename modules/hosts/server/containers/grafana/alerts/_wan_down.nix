# The gateway recorded an internet drop. This is the WAN itself going away, as the UDM
# saw it — not a container, not a service.
{ promRule, ... }:
{
  group = "network";
  rules = [
    (promRule {
      uid = "wan_down";
      title = "WAN drop";
      expr = "increase(unpoller_site_intenet_drops_total[10m])";
      op = "gt";
      value = 0;
      lookback = 1200;
      for = "0m";
      severity = "critical";
      category = "unifi";
      noDataState = "OK";
      summary = "Internet dropped";
      description = "The gateway recorded an internet drop in the last 10m.";
    })
  ];
}
