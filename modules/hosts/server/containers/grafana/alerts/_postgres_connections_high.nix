# Connection pool nearing max_connections (100). Hitting the ceiling doesn't degrade
# gracefully: new connections are refused outright, so every dependent service errors at
# once. Currently sitting at ~33%.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "postgres_connections_high";
      title = "Postgres connections high";
      expr = "sum(pg_stat_activity_count) / scalar(pg_settings_max_connections)";
      op = "gt";
      value = 0.8;
      for = "15m";
      severity = "warning";
      category = "database";
      noDataState = "OK";
      summary = "Postgres above 80% of max_connections";
      description = "Postgres has been above 80% of max_connections for 15m. At 100% new connections are refused, not queued — every dependent service fails at once.";
    })
  ];
}
