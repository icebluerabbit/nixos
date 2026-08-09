# Too many simultaneous transcodes. Each one is a CPU-bound ffmpeg on the server, and
# past a couple they stop keeping up: every stream on the box starts buffering, including
# the ones that were direct-playing happily.
{ promRule, ... }:
{
  group = "media";
  rules = [
    (promRule {
      uid = "jellyfin_transcode_load";
      title = "Jellyfin transcode load";
      expr = "jellyfin_transcodes";
      op = "gt";
      value = 2;
      for = "15m";
      severity = "warning";
      category = "jellyfin";
      noDataState = "OK";
      summary = "Jellyfin running {{ $value }} transcodes";
      description = "Jellyfin has been running more than 2 simultaneous transcodes for 15m. Expect buffering on every stream, not just the transcoded ones.";
    })
  ];
}
