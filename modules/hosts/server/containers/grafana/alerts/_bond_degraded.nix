# The server's bonded trunk lost a member. Everything still works on the surviving
# link, which is exactly why this needs an alert — a silently half-dead bond is only
# discovered when the second link goes too.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "bond_degraded";
      title = "Bond degraded";
      expr = "node_bonding_slaves - node_bonding_active";
      op = "gt";
      value = 0;
      for = "5m";
      severity = "warning";
      category = "host";
      noDataState = "OK";
      summary = "Bond {{ $labels.master }} degraded on {{ $labels.host }}";
      description = "{{ $labels.master }} on {{ $labels.host }} has fewer active slaves than configured — a link is down and the trunk is running without redundancy.";
    })
  ];
}
