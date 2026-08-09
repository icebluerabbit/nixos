{ config, ... }:
{
  name = "Lidarr";
  implementation = "Lidarr";
  tags = [ "\${ref.tag.music}" ];
  sync_level = "fullSync";
  prowlarr_url = "http://${config.infra.clients.container-prowlarr.ip}:${toString config.infra.clients.container-prowlarr.ports.default}";
  base_url = "http://${config.infra.clients.container-lidarr.ip}:${toString config.infra.clients.container-lidarr.ports.default}";
  api_key = "\${env.LIDARR__AUTH__APIKEY}";
  # Prowlarr default Audio (3000) category set — required non-empty.
  sync_categories = [
    3000
    3010
    3030
    3040
    3050
    3060
  ];
}
