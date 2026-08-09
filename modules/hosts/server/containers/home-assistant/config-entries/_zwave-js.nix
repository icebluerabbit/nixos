# Z-Wave JS config entry, pointed at the zwave container's websocket.
{ config, ... }:
{
  entry_id = "7b00f1d1-a06a-43b7-bdd4-a11f74039df7";
  domain = "zwave_js";
  title = "Z-Wave JS";
  # zwave_js sets unique_id = str(controller.home_id) (decimal).
  # 0xd1269f43 = 3508969283 for the NC-ZWA-9734 adapter. A null here
  # gets coerced to the string "None" and trips the migrate_unique_id
  # repair on every start.
  unique_id = "3508969283";
  data.url = "ws://${config.infra.clients.container-zwave.ip}:${toString config.infra.clients.container-zwave.ports.ws}";
}
