# Presence → #clients. person.admin tracks device_tracker.iphone (UniFi); post on
# home↔away transitions. Matrix room membership (ha-bot) is provisioned by tuwunel.
_: {
  id = "presence_admin";
  alias = "Presence: Admin";
  triggers = [
    {
      trigger = "state";
      entity_id = [ "person.admin" ];
      to = "home";
    }
    {
      trigger = "state";
      entity_id = [ "person.admin" ];
      to = "not_home";
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
