# managarr recorder — a SECOND Connect entry alongside _home-assistant.nix,
# not a replacement: that one feeds the Vector sidecar → MQTT → HA path.
{ config, ... }:
let
  m = config.infra.clients.container-managarr;
in
{
  name = "managarr";
  implementation = "Webhook";
  url = "http://${m.ip}:${toString m.ports.default}/hook/radarr";
  method = 1; # POST
  on_grab = true;
  on_download = true;
  on_upgrade = true;
  on_rename = true;
  on_movie_added = true;
  on_movie_delete = true;
  on_movie_file_delete = true;
  # The supersede signal — see the sonarr notifier for why this one matters.
  on_movie_file_delete_for_upgrade = true;
  on_health_issue = true;
  on_health_restored = true;
  on_application_update = true;
  on_manual_interaction_required = true;
  include_health_warnings = true;
}
