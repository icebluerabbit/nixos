# A DHCP pool is running out of addresses. When it empties, new clients simply don't get
# an address — they look "connected" at the wireless layer and have no network.
{ promRule, ... }:
{
  group = "network";
  rules = [
    (promRule {
      uid = "dhcp_pool_low";
      title = "DHCP pool low";
      expr = "unpoller_dhcp_free_percent";
      op = "lt";
      value = 10;
      for = "15m";
      severity = "warning";
      category = "unifi";
      noDataState = "OK";
      summary = "{{ $labels.network }} DHCP pool below 10% free";
      description = "The DHCP pool on {{ $labels.network }} is below 10% free. Once it empties, new clients associate but get no address.";
    })
  ];
}
