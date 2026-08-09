# Prowlarr pushes its indexers into each *arr (fullSync). prowlarr_url is what
# the *arr uses to pull; base_url is where Prowlarr reaches the *arr.
{ config, ... }:
{
  name = "Radarr";
  implementation = "Radarr";
  tags = [ "\${ref.tag.movies}" ];
  sync_level = "fullSync";
  prowlarr_url = "http://${config.infra.clients.container-prowlarr.ip}:${toString config.infra.clients.container-prowlarr.ports.default}";
  base_url = "http://${config.infra.clients.container-radarr.ip}:${toString config.infra.clients.container-radarr.ports.default}";
  api_key = "\${env.RADARR__AUTH__APIKEY}";
  # Prowlarr default Movies (2000) category set — required non-empty.
  sync_categories = [
    2000
    2010
    2020
    2030
    2040
    2045
    2050
    2060
    2070
    2080
    2090
  ];
}
