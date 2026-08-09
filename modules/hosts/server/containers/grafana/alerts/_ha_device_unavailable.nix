# A Home Assistant device dropped off — a presence sensor died, a hub lost its
# connection, the zigbee2mqtt bridge went away. HA shows this in its own UI and nowhere
# else, and for the containers that ship no Vector metrics (zigbee2mqtt among them) this
# is the only health signal that exists at all: target_down cannot see them.
#
# Restricted to binary_sensor and switch, and that restriction is the whole rule.
# homeassistant_entity_available is only trustworthy for entities whose state is a number
# or a boolean; for string-state entities HA's Prometheus exporter reports 0 while the
# entity is perfectly healthy. sensor.zigbee2mqtt_bridge_version ("2.12.1") sat at 0 for
# 40 minutes while zigbee2mqtt was up, connected and serving — as did person.admin
# ("home") and the backup timestamp sensors. A blanket rule over all domains fires on
# those forever and never resolves, and there is no way to tell a string-state entity
# from a numeric one from Prometheus, so excluding them one by one is whack-a-mole.
#
# `light` is deliberately not included either: a bulb cut at the wall switch is genuinely
# unavailable, and that is not an alert.
#
# Second guard: HA's Prometheus exporter is event-driven — it writes
# homeassistant_entity_available only when something happens, never on scrape. A desync
# (MQTT discovery re-add after an HA restart) can pin an entity at 0 with no matching
# state event, and since the entity then never changes state again, the 0 is permanent:
# switch.zigbee2mqtt_bridge_permit_join sat at 0 from 00:41 onward while HA itself
# reported it "off" and healthy and zigbee2mqtt was up and serving.
#
# A real drop always carries a state change (available → unavailable IS a state event),
# so require recent state_change_total activity. Gate window 45m against for=30m leaves
# 15m of firing headroom. Cost: once the drop is >45m old the gate closes and the alert
# resolves on its own, even if the device is still dead — one warning per fault, not a
# standing condition.
{ promRule, ... }:
{
  group = "home";
  rules = [
    (promRule {
      uid = "ha_device_unavailable";
      title = "Home Assistant device unavailable";
      expr = ''
        homeassistant_entity_available{domain=~"binary_sensor|switch"}
          and on (entity)
        changes(homeassistant_state_change_total{domain=~"binary_sensor|switch"}[45m]) > 0
      '';
      op = "lt";
      value = 1;
      lookback = 3600; # must cover the 45m range selector
      for = "30m";
      severity = "warning";
      category = "home";
      noDataState = "OK";
      summary = "{{ $labels.friendly_name }} is unavailable";
      description = "Home Assistant device {{ $labels.entity }} ({{ $labels.friendly_name }}) has been unavailable for 30m — it has stopped answering, or the integration behind it is down.";
    })
  ];
}
