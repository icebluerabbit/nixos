# The UniFi controller stopped answering unpoller. Every network metric in the stack
# comes through this one poller, so when it goes the entire network dashboard goes
# dark — and none of the other UniFi rules below can fire.
{ promRule, ... }:
{
  group = "network";
  rules = [
    (promRule {
      uid = "unifi_controller_down";
      title = "UniFi controller down";
      expr = "unpoller_controller_up";
      op = "lt";
      value = 1;
      for = "5m";
      severity = "critical";
      category = "unifi";
      noDataState = "OK";
      summary = "UniFi controller unreachable";
      description = "unpoller cannot reach the controller at {{ $labels.source }} — all network metrics are stale until it returns.";
    })
  ];
}
