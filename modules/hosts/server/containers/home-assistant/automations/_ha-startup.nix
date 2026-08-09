# HA platform lifecycle → #home-assistant. Startup ping confirms restarts/deploys
# landed. `ha_` prefix marks it infra (excluded from the #automations relay).
_: {
  id = "ha_startup";
  alias = "HA Startup";
  triggers = [
    {
      trigger = "homeassistant";
      event = "start";
    }
  ];
  conditions = [ ];
  actions = [
    {
      action = "matrix.send_message";
      data = {
        message = builtins.readFile ./msg/home-assistant/Startup.tmpl;
        data.format = "html";
        target = [ "#home-assistant:matrix.services.internal" ];
      };
    }
  ];
  mode = "single";
}
