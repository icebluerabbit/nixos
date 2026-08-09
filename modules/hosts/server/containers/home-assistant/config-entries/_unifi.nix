# UniFi controller config entry (device trackers, bandwidth/uptime sensors).
{ config, ... }:
{
  # Credentials expand from seedEnvironmentFile at seed time; the Nix
  # store only ever sees the literal $UNIFI_* references.
  entry_id = "d916326e-620a-40bc-b00c-d1a4892c3d2e";
  domain = "unifi";
  title = "UniFi";
  # UniFi hub asserts entry.unique_id; it must be the controller's
  # internal site _id (from /proxy/network/api/self/sites), not the
  # site "name". This is the default site on the UDM Pro Max.
  unique_id = "6a20c67aaa20ab3db2a16682";
  data = {
    host = config.infra.vlans.iot.staticAddresses.gateway;
    username = "$UNIFI_USERNAME";
    password = "$UNIFI_PASSWORD";
    port = 443;
    site = "default";
    verify_ssl = false;
  };
  options = {
    allow_bandwidth_sensors = true;
    allow_uptime_sensors = true;
    block_client = [ ];
    client_source = [ ];
    detection_time = 300;
    dpi_restrictions = true;
    ignore_wired_bug = false;
    ssid_filter = [
      "herp"
      "derp"
      "werp"
    ];
    track_clients = true;
    track_devices = true;
    track_wired_clients = true;
  };
}
