# A Vector component is erroring — a sink that can't reach its destination, a transform
# that can't parse. Pairs with vector_dropping_events: that one catches telemetry being
# thrown away, this one catches the pipeline breaking before it gets that far.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "vector_component_errors";
      title = "Vector component errors";
      expr = "sum by (host, component_id) (rate(vector_component_errors_total[10m])) * 600";
      op = "gt";
      value = 10;
      lookback = 1200;
      for = "15m";
      severity = "warning";
      category = "telemetry";
      noDataState = "OK";
      summary = "Vector {{ $labels.component_id }} erroring on {{ $labels.host }}";
      description = "The Vector component {{ $labels.component_id }} on {{ $labels.host }} is throwing errors — its telemetry is not arriving.";
    })
  ];
}
