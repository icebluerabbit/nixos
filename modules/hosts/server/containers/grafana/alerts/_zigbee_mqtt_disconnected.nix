# zigbee2mqtt lost its MQTT broker connection. The radio may be fine and devices may
# still be talking to it, but nothing reaches Home Assistant — z2m is the only path.
{ lokiRule, ... }:
{
  group = "home";
  rules = [
    (lokiRule {
      uid = "zigbee_mqtt_disconnected";
      title = "zigbee2mqtt MQTT disconnected";
      expr = ''sum(count_over_time({host="zigbee2mqtt"} | json | message =~ `.*"connected":false.*` [15m]))'';
      op = "gt";
      value = 0;
      lookback = 1800;
      for = "5m";
      severity = "critical";
      category = "home";
      noDataState = "OK";
      keepFiringFor = "15m";
      summary = "zigbee2mqtt is disconnected from the broker";
      description = "zigbee2mqtt reports mqtt.connected=false. Devices may still be reporting to the bridge, but nothing is reaching mosquitto or Home Assistant.";
    })
  ];
}
