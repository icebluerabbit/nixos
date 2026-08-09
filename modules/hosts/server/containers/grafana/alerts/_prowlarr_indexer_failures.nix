# Prowlarr's indexer queries are failing. Every arr searches through Prowlarr, so a
# failing indexer degrades all of them at once, and the only visible symptom is that
# nothing gets found any more.
{ promRule, ... }:
{
  group = "media";
  rules = [
    (promRule {
      uid = "prowlarr_indexer_failures";
      title = "Prowlarr indexer failures";
      expr = "increase(prowlarr_indexer_failed_queries_total[1h])";
      op = "gt";
      value = 5;
      lookback = 7200;
      for = "15m";
      severity = "warning";
      category = "prowlarr";
      noDataState = "OK";
      keepFiringFor = "30m";
      summary = "Prowlarr indexer failing ({{ $labels.indexer }})";
      description = "{{ $labels.indexer }} has failed more than 5 queries in the last hour. Searches through it are coming back empty for every arr.";
    })
  ];
}
