{ config, ... }:
{
  name = "Lidarr";
  client_type = "LIDARR";
  enabled = true;
  host = "http://${config.infra.clients.container-lidarr.ip}:${toString config.infra.clients.container-lidarr.ports.default}";
  settings.apikey = "\${env.LIDARR_APIKEY}";
}
