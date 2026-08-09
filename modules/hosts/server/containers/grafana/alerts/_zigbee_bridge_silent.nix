# zigbee2mqtt publishes a bridge/health payload every 10 minutes. When those stop, the
# process is wedged or the radio is gone — and because z2m exports no Prometheus metrics,
# target_down only sees the container, which stays up either way.
#
# Absence is the signal, so noDataState is Alerting: an empty result means no health
# publish reached Loki at all, which is the fault this rule exists to catch.
{ lokiRule, ... }:
{
  group = "home";
  rules = [
    (lokiRule {
      uid = "zigbee_bridge_silent";
      title = "zigbee2mqtt bridge silent";
      expr = ''sum(count_over_time({host="zigbee2mqtt"} | json | message =~ `.*bridge/health.*` [30m]))'';
      op = "lt";
      value = 1;
      lookback = 3600;
      for = "10m";
      severity = "critical";
      category = "home";
      noDataState = "Alerting";
      summary = "zigbee2mqtt has stopped reporting health";
      description = "No zigbee2mqtt bridge/health publish in 30m — it publishes every 10m, so three are missing. The process is wedged or the coordinator has dropped off; every Zigbee device is unreachable regardless of what Home Assistant still shows.";
    })
  ];
}
