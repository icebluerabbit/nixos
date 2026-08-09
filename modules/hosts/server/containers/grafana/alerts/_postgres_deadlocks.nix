# Deadlocks are being recorded. Postgres resolves each one by killing a transaction, so
# the database stays up and some caller silently lost a write.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "postgres_deadlocks";
      title = "Postgres deadlocks";
      expr = "increase(pg_stat_database_deadlocks[1h])";
      op = "gt";
      value = 0;
      lookback = 7200;
      for = "5m";
      severity = "warning";
      category = "database";
      noDataState = "OK";
      summary = "Postgres deadlocks on {{ $labels.datname }}";
      description = "Postgres killed a transaction to break a deadlock on {{ $labels.datname }}. Whoever was holding it lost its write and probably didn't notice.";
    })
  ];
}
