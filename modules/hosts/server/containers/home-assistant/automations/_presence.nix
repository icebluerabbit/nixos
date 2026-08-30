# Presence → #clients. person.admin tracks device_tracker.iphone (UniFi); post on
# home↔away transitions. Matrix room membership (ha-bot) is provisioned by tuwunel.
_: {
  id = "presence_admin";
  alias = "Presence: Admin";
  # not_from/for guard the UniFi tracker: when the controller reboots or its API
  # 502s, device_tracker.iphone goes unavailable and person.admin drops to unknown,
  # so a bare `to = "home"` reads every recovery as an arrival (20 false posts on
  # 2026-08-30). Cost: an arrival during a controller outage goes unannounced.
  triggers = [
    {
      trigger = "state";
      entity_id = [ "person.admin" ];
      to = "home";
      not_from = [
        "unknown"
        "unavailable"
      ];
      for = "00:02:00";
    }
    {
      trigger = "state";
      entity_id = [ "person.admin" ];
      to = "not_home";
      not_from = [
        "unknown"
        "unavailable"
      ];
      for = "00:02:00";
    }
  ];
  conditions = [ ];
  actions = [
    {
      action = "matrix.send_message";
      data = {
        message = "{% if trigger.to_state.state == 'home' %}🏠 <b>Admin</b> arrived home{% else %}🚶 <b>Admin</b> left home{% endif %}";
        data.format = "html";
        target = [ "#clients:matrix.services.internal" ];
      };
    }
  ];
  mode = "queued";
}
