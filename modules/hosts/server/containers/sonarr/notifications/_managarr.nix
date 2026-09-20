# managarr recorder — a SECOND Connect entry alongside _home-assistant.nix,
# not a replacement: that one feeds the Vector sidecar → MQTT → HA path, and
# repointing it would silently kill the notifications.
#
# Everything is on, including on_import_complete, which the HA notifier
# deliberately disables. There it renders as an empty duplicate card; here a
# second payload shape for the same event is the point — it carries an
# episodeFiles array where on_download carries a singular episodeFile, and
# phase two needs to know which one to key the torrent→library mapping on.
{ config, ... }:
let
  m = config.infra.clients.container-managarr;
in
{
  name = "managarr";
  implementation = "Webhook";
  url = "http://${m.ip}:${toString m.ports.default}/hook/sonarr";
  method = 1; # POST
  on_grab = true;
  on_download = true;
  on_import_complete = true;
  on_upgrade = true;
  on_rename = true;
  on_series_add = true;
  on_series_delete = true;
  on_episode_file_delete = true;
  # The supersede signal: fires when an existing file is removed because a
  # better release replaced it. This is the event that answers "is the content
  # still in the library" without counting hardlinks.
  on_episode_file_delete_for_upgrade = true;
  on_health_issue = true;
  on_health_restored = true;
  on_application_update = true;
  on_manual_interaction_required = true;
  include_health_warnings = true;
}
