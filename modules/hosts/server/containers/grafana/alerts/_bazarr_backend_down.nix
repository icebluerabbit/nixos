# Bazarr lost its connection to Sonarr or Radarr. Bazarr keeps running and its UI stays
# up; it simply stops learning about new media, so subtitles quietly stop appearing for
# anything added after the link broke.
{ promRule, ... }:
let
  rule =
    app:
    promRule {
      uid = "bazarr_${app}_down";
      title = "Bazarr cannot reach ${app}";
      expr = "bazarr_${app}_up";
      op = "lt";
      value = 1;
      for = "15m";
      severity = "warning";
      category = "bazarr";
      noDataState = "OK";
      summary = "Bazarr cannot reach ${app}";
      description = "Bazarr has been unable to reach ${app} for 15m — it will not see new media, and subtitles will silently stop being fetched.";
    };
in
{
  group = "media";
  rules = map rule [
    "sonarr"
    "radarr"
  ];
}
