_: {
  id = "moes_button_3_living_100";
  alias = "Living Room Lights - Button 3 - Toggle 100%";
  triggers = [
    {
      domain = "mqtt";
      device_id = "0061a9f813d83519b43041966bc0cfcb";
      type = "action";
      subtype = "3_single";
      trigger = "device";
    }
  ];
  conditions = [ ];
  actions = [
    {
      action = "light.toggle";
      metadata = { };
      target.entity_id = "light.living_living";
      data.brightness_pct = 100;
    }
  ];
  mode = "single";
}
