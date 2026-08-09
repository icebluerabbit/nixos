{ config, ... }:
{
  name = "Radarr";
  client_type = "RADARR";
  enabled = true;
  host = "http://${config.infra.clients.container-radarr.ip}:${toString config.infra.clients.container-radarr.ports.default}";
  settings.apikey = "\${env.RADARR_APIKEY}";
}
