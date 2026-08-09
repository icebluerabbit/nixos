# Non-webhook automation activity → #automations. Fires on HA's automation_triggered
# event; excludes the *_webhook notification bridges + ha_* platform automations (they
# post to their own rooms) and this relay itself (else it self-triggers into a loop).
# The event only carries name/entity_id/source — no outcome — so it reports name + trigger.
_: {
  id = "automations_relay";
  alias = "Automations Relay";
  triggers = [
    {
      trigger = "event";
      event_type = "automation_triggered";
    }
  ];
  conditions = [
    {
      condition = "template";
      value_template = "{{ not trigger.event.data.entity_id.endswith('_webhook') and not trigger.event.data.entity_id.startswith('automation.ha_') and trigger.event.data.entity_id != 'automation.automations_relay' }}";
    }
  ];
  actions = [
    {
      action = "matrix.send_message";
      data = {
        message = builtins.readFile ./msg/automations/Triggered.tmpl;
        data.format = "html";
        target = [ "#automations:matrix.services.internal" ];
      };
    }
  ];
  mode = "queued";
}
