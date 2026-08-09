# Jellyfin webhook → Matrix. One Generic Destination (jellyfin-plugin-webhook,
# "Send All Properties") POSTs here; branch on NotificationType. Security events
# fan out to #alerts, everything else lands in #jellyfin.
_:
let
  jellyfin = [ "#jellyfin:matrix.services.internal" ];
  alerts = [ "#alerts:matrix.services.internal" ];
  branch = event: tmpl: target: {
    conditions = [
      {
        condition = "template";
        value_template = "{{ trigger.payload_json.NotificationType == '${event}' }}";
      }
    ];
    sequence = [
      {
        action = "matrix.send_message";
        data = {
          message = builtins.readFile tmpl;
          data.format = "html";
          inherit target;
        };
      }
    ];
  };
in
{
  id = "jellyfin_webhook";
  alias = "Jellyfin Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/jellyfin";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [
        # Security → #alerts.
        (branch "AuthenticationFailure" ./msg/jellyfin/AuthenticationFailure.tmpl alerts)
        (branch "UserLockedOut" ./msg/jellyfin/UserLockedOut.tmpl alerts)
        (branch "UserPasswordChanged" ./msg/jellyfin/UserPasswordChanged.tmpl alerts)

        # Everything else → #jellyfin.
        (branch "AuthenticationSuccess" ./msg/jellyfin/AuthenticationSuccess.tmpl jellyfin)
        (branch "ItemAdded" ./msg/jellyfin/ItemAdded.tmpl jellyfin)
        (branch "ItemDeleted" ./msg/jellyfin/ItemDeleted.tmpl jellyfin)
        (branch "PendingRestart" ./msg/jellyfin/PendingRestart.tmpl jellyfin)
        (branch "PlaybackProgress" ./msg/jellyfin/PlaybackProgress.tmpl jellyfin)
        (branch "PlaybackStart" ./msg/jellyfin/PlaybackStart.tmpl jellyfin)
        (branch "PlaybackStop" ./msg/jellyfin/PlaybackStop.tmpl jellyfin)
        (branch "PluginInstallationCancelled" ./msg/jellyfin/PluginInstallationCancelled.tmpl jellyfin)
        (branch "PluginInstallationFailed" ./msg/jellyfin/PluginInstallationFailed.tmpl jellyfin)
        (branch "PluginInstalled" ./msg/jellyfin/PluginInstalled.tmpl jellyfin)
        (branch "PluginInstalling" ./msg/jellyfin/PluginInstalling.tmpl jellyfin)
        (branch "PluginUninstalled" ./msg/jellyfin/PluginUninstalled.tmpl jellyfin)
        (branch "PluginUpdated" ./msg/jellyfin/PluginUpdated.tmpl jellyfin)
        (branch "SessionStart" ./msg/jellyfin/SessionStart.tmpl jellyfin)
        (branch "SubtitleDownloadFailure" ./msg/jellyfin/SubtitleDownloadFailure.tmpl jellyfin)
        (branch "TaskCompleted" ./msg/jellyfin/TaskCompleted.tmpl jellyfin)
        (branch "UserCreated" ./msg/jellyfin/UserCreated.tmpl jellyfin)
        (branch "UserDataSaved" ./msg/jellyfin/UserDataSaved.tmpl jellyfin)
        (branch "UserDeleted" ./msg/jellyfin/UserDeleted.tmpl jellyfin)
        (branch "UserUpdated" ./msg/jellyfin/UserUpdated.tmpl jellyfin)
      ];
      default = [
        {
          action = "matrix.send_message";
          data = {
            message = "{{ trigger.payload_json | tojson(indent=2) }}";
            target = jellyfin;
          };
        }
      ];
    }
  ];
  mode = "queued";
}
