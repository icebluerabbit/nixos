# MQTT broker (mosquitto) config entry.
{ config, ... }:
{
  entry_id = "6c8cd798-96e9-42ee-b236-f89c2f922984";
  domain = "mqtt";
  title = "MQTT";
  data = {
    broker = config.infra.clients.container-mosquitto.ip;
    port = config.infra.clients.container-mosquitto.ports.default;
    client_id = "home-assistant";
    protocol = "5";
  };
}
