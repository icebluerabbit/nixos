# Grafana alerts → Matrix. Grafana's webhook contact point POSTs alert batches to the
# grafana Vector sidecar, which explodes alerts[] into one MQTT message per alert on
# homelab/notify/grafana/alerts/<alertname>. One wildcard subscription; the choose routes
# on the alert's own labels.
#
# Severity-first. Every rule declares what it is about (`category`) and how badly
# (`severity`); this file decides where that gets said:
#
#   critical   goes to #alerts and nothing else. #alerts is the severity view —
#              "what is broken right now" — which is what tuwunel.nix files it under.
#   anything   else routes to the room for the subsystem it is about, making that room
#   else       the full history of that service. A category with no room of its own
#              lands in #alerts rather than being dropped.
#
# A critical short-circuits before category routing, so it is said once, in #alerts —
# never mirrored into a service room. Neither side needs to know the other's names: a
# new alert in an existing category routes itself.
{ lib }:
let
  # room → the categories that belong in it. Several categories share #nixos: from a
  # notification standpoint the host, its disks, the database, the proxy and the telemetry
  # pipeline are all "the infrastructure broke".
  rooms = {
    nixos = [
      "host"
      "storage"
      "database"
      "proxy"
      "telemetry"
    ];
    unifi = [ "unifi" ];
    dns = [ "dns" ];
    vpn-gw = [ "vpn" ];
    radarr = [ "radarr" ];
    sonarr = [ "sonarr" ];
    prowlarr = [ "prowlarr" ];
    bazarr = [ "bazarr" ];
    autobrr = [ "autobrr" ];
    qbittorrent = [ "qbittorrent" ];
    cross-seed = [ "cross-seed" ];
    soulseek = [ "soulseek" ];
    jellyfin = [ "jellyfin" ];
    home-assistant = [ "home" ];
  };

  send = room: {
    action = "matrix.send_message";
    data = {
      message = builtins.readFile ./msg/grafana/Alert.tmpl;
      data.format = "html";
      target = [ "#${room}:matrix.services.internal" ];
    };
  };

  branch = room: categories: {
    conditions = [
      {
        condition = "template";
        value_template = "{{ trigger.payload_json.labels.category in ${builtins.toJSON categories} }}";
      }
    ];
    sequence = [ (send room) ];
  };

  # Matched before any category branch, so a critical is said once — in #alerts — and
  # never also mirrored into its service room.
  criticalBranch = {
    conditions = [
      {
        condition = "template";
        value_template = "{{ trigger.payload_json.labels.severity == 'critical' }}";
      }
    ];
    sequence = [ (send "alerts") ];
  };
in
{
  id = "grafana_alerts_webhook";
  alias = "Grafana Alerts Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/grafana/alerts/+";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [ criticalBranch ] ++ lib.mapAttrsToList branch rooms;
      # Unroomed non-critical — a category with no room of its own — lands here.
      default = [ (send "alerts") ];
    }
  ];
  mode = "queued";
}
