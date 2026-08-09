# The arrs' own health checks. exportarr lifts each issue into a series whose `message`
# label is the human-readable text, so the notification can just quote it — indexer
# failing, download client unreachable, root folder missing, permissions wrong.
#
# source!="UpdateCheck" is the whole reason this rule is usable. radarr and sonarr both
# hold a permanent "New update is available" health issue, and a bare `> 0` would fire
# on both forever and say nothing. Update nags are not faults.
{ promRule, ... }:
let
  rule =
    app:
    promRule {
      uid = "${app}_health_issue";
      title = "${app} health issue";
      expr = ''${app}_system_health_issues{source!="UpdateCheck"}'';
      op = "gt";
      value = 0;
      for = "15m";
      severity = "warning";
      category = app;
      noDataState = "OK";
      summary = "${app}: {{ $labels.message }}";
      description = "${app} is reporting a health issue ({{ $labels.source }}): {{ $labels.message }}";
    };
in
{
  group = "media";
  rules = map rule [
    "radarr"
    "sonarr"
    "lidarr"
    "prowlarr"
  ];
}
