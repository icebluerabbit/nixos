# Loki's write-ahead log cannot write to disk. Loki is where every log in the fleet
# lands, so this is the point at which the homelab stops being able to explain itself.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "loki_wal_disk_full";
      title = "Loki WAL disk full";
      expr = "increase(loki_ingester_wal_disk_full_failures_total[10m])";
      op = "gt";
      value = 0;
      lookback = 1200;
      for = "5m";
      severity = "critical";
      category = "telemetry";
      noDataState = "OK";
      summary = "Loki cannot write its WAL";
      description = "Loki's ingester WAL is failing to write — the disk is full. Logs are being lost fleet-wide.";
    })
  ];
}
