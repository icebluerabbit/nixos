# Lidarr webhook → #lidarr Matrix room. Own webhook_id = sender identity,
# so routing needs no instanceName check; branch only picks the event template.
_:
let
  target = [ "#lidarr:matrix.services.internal" ];
  branch = event: tmpl: {
    conditions = [
      {
        condition = "template";
        value_template = "{{ trigger.payload_json.eventType == '${event}' }}";
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
  id = "lidarr_webhook";
  alias = "Lidarr Webhook";
  triggers = [
    {
      trigger = "mqtt";
      topic = "homelab/notify/lidarr";
      qos = 1;
    }
  ];
  conditions = [ ];
  actions = [
    {
      choose = [
        (branch "Test" ./msg/lidarr/Test.tmpl)
        (branch "Grab" ./msg/lidarr/Grab.tmpl)
        (branch "Download" ./msg/lidarr/Download.tmpl)
        (branch "ArtistAdd" ./msg/lidarr/ArtistAdd.tmpl)
        (branch "ArtistDelete" ./msg/lidarr/ArtistDelete.tmpl)
        (branch "AlbumAdd" ./msg/lidarr/AlbumAdd.tmpl)
        (branch "AlbumDelete" ./msg/lidarr/AlbumDelete.tmpl)
        (branch "TrackRetag" ./msg/lidarr/TrackRetag.tmpl)
        (branch "Rename" ./msg/lidarr/Rename.tmpl)
        (branch "Health" ./msg/lidarr/Health.tmpl)
        (branch "HealthRestored" ./msg/lidarr/HealthRestored.tmpl)
        (branch "ApplicationUpdate" ./msg/lidarr/ApplicationUpdate.tmpl)
      ];
      default = [
        {
          action = "matrix.send_message";
          data = {
            message = "{{ trigger.payload_json | tojson(indent=2) }}";
            inherit target;
          };
        }
      ];
    }
  ];
  mode = "queued";
}
