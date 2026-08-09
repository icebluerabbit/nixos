# dnsmasq's upstream servers are failing queries. DNS breaking is the failure that
# looks like every other failure at once: nothing resolves, so everything "is down".
{ promRule, ... }:
{
  group = "network";
  rules = [
    (promRule {
      uid = "dns_upstream_failures";
      title = "DNS upstream failures";
      expr = "sum(rate(dnsmasq_servers_queries_failed[5m])) * 60";
      op = "gt";
      value = 5;
      for = "10m";
      severity = "warning";
      category = "dns";
      noDataState = "OK";
      summary = "dnsmasq upstreams failing >5 queries/min";
      description = "dnsmasq's upstream resolvers have been failing more than 5 queries per minute for 10m.";
    })
  ];
}
