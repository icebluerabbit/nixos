# Firmware update-available → #home-assistant. Watches the device update.* entities
# (no HA-core update entity exists — Core-in-nspawn, no supervisor). Enumerated
# because state triggers can't wildcard a domain; add new updatable devices here.
_: {
  id = "ha_update_available";
  alias = "HA Update Available";
  triggers = [
    {
      trigger = "state";
      entity_id = [
        "update.bedroom_temp_sensor"
        "update.living_room_temp_sensor"
        "update.server_rack_z_wave_controller_firmware"
      ];
      to = "on";
    }
  ];
  conditions = [ ];
  actions = [
    {
      action = "matrix.send_message";
      data = {
        message = builtins.readFile ./msg/home-assistant/UpdateAvailable.tmpl;
        data.format = "html";
        target = [ "#home-assistant:matrix.services.internal" ];
      };
    }
  ];
  mode = "queued";
}
