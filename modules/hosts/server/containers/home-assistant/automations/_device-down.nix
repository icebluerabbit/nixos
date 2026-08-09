# UniFi infra device down → #unifi. The 4 UniFi devices each have a UniFi-integration
# device_tracker; it goes `not_home` when the controller marks the device disconnected.
# 5m debounce on down (survives a reboot/blip), 1m on recovery. Distinct from
# unifi_controller_down (unpoller ↔ controller) — this is a single AP/switch/gateway
# dropping while the controller is still reporting.
_:
let
  devices = [
    "device_tracker.dream_machine_pro_max"
    "device_tracker.u7_pro_xg"
    "device_tracker.usw_pro_hd_24_poe"
    "device_tracker.usw_flex_2_5g_8"
  ];
in
{
  id = "network_device_down";
  alias = "Network: UniFi Device Down";
  triggers = [
    {
      trigger = "state";
      entity_id = devices;
      to = "not_home";
      for = "00:05:00";
    }
    {
      trigger = "state";
      entity_id = devices;
      to = "home";
      for = "00:01:00";
    }
  ];
  conditions = [ ];
  actions = [
    {
      action = "matrix.send_message";
      data = {
        message = "{% set n = trigger.to_state.attributes.friendly_name | default(trigger.entity_id) %}{% if trigger.to_state.state == 'home' %}✅ <b>UniFi device back online:</b> {{ n }}{% else %}🔴 <b>UniFi device DOWN:</b> {{ n }} — offline >5m{% endif %}";
        data.format = "html";
        target = [ "#unifi:matrix.services.internal" ];
      };
    }
  ];
  mode = "queued";
}
