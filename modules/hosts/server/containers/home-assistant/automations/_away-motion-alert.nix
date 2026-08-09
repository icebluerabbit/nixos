# Away + motion → #alerts. Either FP2 presence sensor tripping while admin is
# not_home, 09:00–18:00, posts a possible-intrusion alert. person.admin tracks
# device_tracker.iphone (see _presence.nix); FP2 presence == occupancy/motion.
_:
let
  presence = [
    "binary_sensor.living_room_presence_sensor_fp2_db2e_living_room_presence_sensor_1"
    "binary_sensor.bedroom_presence_sensor_fp2_96e8_bedroom_presence_sensor_1"
  ];
in
{
  id = "security_away_motion";
  alias = "Security: Motion While Away";
  triggers = [
    {
      trigger = "state";
      entity_id = presence;
      to = "on";
    }
  ];
  conditions = [
    {
      condition = "state";
      entity_id = "person.admin";
      state = "not_home";
    }
    {
      condition = "time";
      after = "09:00:00";
      before = "18:00:00";
    }
  ];
  actions = [
    {
      action = "matrix.send_message";
      data = {
        message = "{% set n = trigger.to_state.attributes.friendly_name | default(trigger.entity_id) %}🚨 <b>Motion while away:</b> {{ n }}";
        data.format = "html";
        target = [ "#alerts:matrix.services.internal" ];
      };
    }
  ];
  mode = "queued";
}
