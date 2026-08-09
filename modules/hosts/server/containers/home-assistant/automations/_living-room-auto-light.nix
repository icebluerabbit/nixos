# Evening auto-light. Living-room FP2 motion while admin home, dim (lux < 15; FP2
# daylight peaks ~25–85, lights-on reads ~5–9), Living Room group off, 17:00–24:00
# → turn the group on. Motion-driven, so covers arrival and later trips into a dark
# room; the lights-off guard means it won't re-fight a manual off until next motion.
_: {
  id = "living_room_evening_autolight";
  alias = "Living Room: Evening Auto-Light";
  triggers = [
    {
      trigger = "state";
      entity_id = "binary_sensor.living_room_presence_sensor_fp2_db2e_living_room_presence_sensor_1";
      to = "on";
    }
  ];
  conditions = [
    {
      condition = "state";
      entity_id = "person.admin";
      state = "home";
    }
    {
      condition = "state";
      entity_id = "light.living_living";
      state = "off";
    }
    {
      condition = "numeric_state";
      entity_id = "sensor.living_room_presence_sensor_fp2_db2e_living_room_light_sensor_light_level";
      below = 15;
    }
    {
      condition = "time";
      after = "17:00:00";
      before = "23:59:59";
    }
  ];
  actions = [
    {
      action = "light.turn_on";
      target.entity_id = "light.living_living";
    }
  ];
  mode = "single";
}
