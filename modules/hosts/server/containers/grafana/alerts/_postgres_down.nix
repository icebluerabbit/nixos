# Postgres itself is down while its container is still up — distinct from the container
# dying, which target_down already covers. Grafana, Authelia and Gitea all keep their
# state here, so this takes them with it.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "postgres_down";
      title = "Postgres down";
      expr = "pg_up";
      op = "lt";
      value = 1;
      for = "5m";
      severity = "critical";
      category = "database";
      noDataState = "OK";
      summary = "Postgres is not accepting connections";
      description = "pg_up is 0 — the postgres container is running but the database is not answering. Everything that stores state here (Grafana, Authelia, Gitea) is down with it.";
    })
  ];
}
