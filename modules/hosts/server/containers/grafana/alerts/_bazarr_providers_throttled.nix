# Subtitle providers throttling Bazarr. Two are throttled right now, which is why the
# `for` is six hours rather than minutes: providers throttle on a daily quota and clear
# themselves, and that is routine. What is not routine is a provider that never comes
# back — that means subtitles have stopped and nothing will say so.
{ promRule, ... }:
{
  group = "media";
  rules = [
    (promRule {
      uid = "bazarr_providers_throttled";
      title = "Bazarr providers throttled";
      expr = "bazarr_providers_throttled";
      op = "gt";
      value = 0;
      for = "6h";
      severity = "warning";
      category = "bazarr";
      noDataState = "OK";
      summary = "Bazarr providers throttled for 6h";
      description = "Bazarr has had throttled subtitle providers for 6h straight. A daily quota clears on its own — this hasn't.";
    })
  ];
}
