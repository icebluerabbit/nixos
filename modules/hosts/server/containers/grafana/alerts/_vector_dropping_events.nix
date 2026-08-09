# Vector is discarding events it did not mean to discard. Every sidecar buffers with
# when_full=drop_newest, so when a sink is slow or unreachable the telemetry is thrown
# away rather than delayed — and the failure mode of an observability stack is that it
# stops observing without telling anyone. This is the watchdog on the watchdog.
#
# intentional="false" only, and that qualifier is load-bearing: the drop counter is
# permanently nonzero with intentional="true" on server, desktop, qbittorrent, vpn-gw,
# home-assistant, postgresql, grafana and unifi-exporter — the prometheus_remote_write
# memory buffer overflowing and shedding metrics by design (~250k events / 10m on
# server alone). That is real data loss, but it is a buffer-sizing problem, not an
# alertable event: matching it here would fire on eight targets forever and train
# everyone to ignore the alert. Fix the buffers; alert on the drops nobody chose.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "vector_dropping_events";
      title = "Vector dropping events";
      expr = ''increase(vector_buffer_discarded_events_total{intentional="false"}[10m])'';
      op = "gt";
      value = 0;
      lookback = 1200;
      for = "10m";
      severity = "warning";
      category = "telemetry";
      noDataState = "OK";
      summary = "Vector on {{ $labels.host }} is dropping events";
      description = "The Vector buffer on {{ $labels.host }} is discarding events unintentionally — logs or metrics from this target are being lost, not delayed.";
    })
  ];
}
