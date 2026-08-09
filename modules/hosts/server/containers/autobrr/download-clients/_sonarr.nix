# autobrr forwards matched announces to the *arr, which decides quality/dedup
# and grabs via its own download client. For arr clients autobrr uses host as the
# full base URL and ignores the separate port field — so the port must be baked
# into host, else it dials :80 and times out.
{ config, ... }:
{
  name = "Sonarr";
  client_type = "SONARR";
  enabled = true;
  host = "http://${config.infra.clients.container-sonarr.ip}:${toString config.infra.clients.container-sonarr.ports.default}";
  settings.apikey = "\${env.SONARR_APIKEY}";
}
