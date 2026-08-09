_: {
  id = "moes_button_2_living_66";
  alias = "Living Room Lights - Button 2 - Toggle 66%";
  triggers = [
    {
      domain = "mqtt";
      device_id = "0061a9f813d83519b43041966bc0cfcb";
      type = "action";
      subtype = "2_single";
      trigger = "device";
    }
  ];
  conditions = [ ];
  actions = [
    {
      action = "light.toggle";
      metadata = { };
      target.entity_id = "light.living_living";
      data.brightness_pct = 66;
    }
  ];
  mode = "single";
}
