{ config, ... }:
{
  name = "Sonarr";
  implementation = "Sonarr";
  tags = [ "\${ref.tag.tv}" ];
  sync_level = "fullSync";
  prowlarr_url = "http://${config.infra.clients.container-prowlarr.ip}:${toString config.infra.clients.container-prowlarr.ports.default}";
  base_url = "http://${config.infra.clients.container-sonarr.ip}:${toString config.infra.clients.container-sonarr.ports.default}";
  api_key = "\${env.SONARR__AUTH__APIKEY}";
  # Prowlarr default TV (5000) set, plus 5070 (TV/Anime) and 5080 (TV/Documentary)
  # for RetroToon. Prowlarr intersects this against each indexer's advertised caps,
  # so an indexer only receives the categories it actually supports.
  #
  # anime_sync_categories is deliberately left unset. Sonarr uses an indexer's Anime
  # Categories *instead of* its Categories for seriesType=anime, and Prowlarr pushes
  # the anime set to every synced indexer without intersecting it against caps. Only
  # TorrentLeech and RetroToon advertise 5070 — setting it took PreToMe from 25
  # results to 0 for all 80 anime series (DigitalCore would do the same). Leaving it
  # empty makes anime fall back to the list above, which IS intersected per indexer,
  # so each tracker gets exactly the categories it really has.
  sync_categories = [
    5000
    5010
    5020
    5030
    5040
    5045
    5050
    5070
    5080
    5090
  ];
}
