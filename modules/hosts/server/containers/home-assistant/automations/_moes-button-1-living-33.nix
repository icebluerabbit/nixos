_: {
  id = "moes_button_1_living_33";
  alias = "Living Room Lights - Button 1 - Toggle 33%";
  triggers = [
    {
      domain = "mqtt";
      device_id = "0061a9f813d83519b43041966bc0cfcb";
      type = "action";
      subtype = "1_single";
      trigger = "device";
    }
  ];
  conditions = [ ];
  actions = [
    {
      action = "light.toggle";
      metadata = { };
      target.entity_id = "light.living_living";
      data.brightness_pct = 33;
    }
  ];
  mode = "single";
}
