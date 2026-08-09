# New client on the network → #clients. The UniFi integration registers a
# device_tracker the first time it sees a client; entity_registry_updated/create
# for a device_tracker.* is that first-sighting. Small delay lets the tracker's
# ip/mac attributes populate before we render. Fires for new container trackers
# too (rare, and worth knowing).
_: {
  id = "network_new_client";
  alias = "Network: New Client";
  triggers = [
    {
      trigger = "event";
      event_type = "entity_registry_updated";
      event_data.action = "create";
    }
  ];
  conditions = [
    {
      condition = "template";
      value_template = "{{ trigger.event.data.entity_id.startswith('device_tracker.') }}";
    }
  ];
  actions = [
    { delay = "00:00:05"; }
    {
      action = "matrix.send_message";
      data = {
        message = ''
          {% set e = trigger.event.data.entity_id %}🆕 <b>New client on network</b>
          <ul>
          <li><b>Name:</b> {{ state_attr(e, 'friendly_name') | default(e) }}</li>
          <li><b>IP:</b> {{ state_attr(e, 'ip') | default('—') }}</li>
          <li><b>MAC:</b> {{ state_attr(e, 'mac') | default('—') }}</li>
          <li><b>Hostname:</b> {{ state_attr(e, 'hostname') | default('—') }}</li>
          <li><b>Entity:</b> <code>{{ e }}</code></li>
          </ul>'';
        data.format = "html";
        target = [ "#clients:matrix.services.internal" ];
      };
    }
  ];
  mode = "queued";
}
