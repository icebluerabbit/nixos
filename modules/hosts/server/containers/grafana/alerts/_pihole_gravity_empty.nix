# Gravity is the blocklist database, and Pi-hole answers queries perfectly well
# without one — it just stops blocking. That silence is the whole problem: an empty
# gravity.db looks identical to a healthy resolver from every other angle. This
# container ran two weeks in exactly that state before the lists option was set.
{ promRule, ... }:
{
  group = "network";
  rules = [
    (promRule {
      uid = "pihole_gravity_empty";
      title = "Pi-hole gravity list empty";
      expr = "pihole_domains_being_blocked";
      op = "lt";
      value = 1000;
      for = "15m";
      severity = "warning";
      category = "dns";
      # A missing series here means the exporter is gone, which pihole_down already
      # covers — don't double-report the same outage.
      noDataState = "OK";
      summary = "Pi-hole gravity list is empty or tiny";
      description = "pihole_domains_being_blocked is under 1000 — gravity.db failed to build or was wiped. Pi-hole is resolving normally but blocking nothing.";
    })
  ];
}
