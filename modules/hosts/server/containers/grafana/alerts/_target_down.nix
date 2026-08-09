# Liveness. Prometheus here is fed by Vector remote_write, not by scraping, so there is
# no `up` series and no scrape-target health to alert on: the only thing that says "this
# box is alive" is that its own Vector sidecar is still publishing. Every host and every
# container ships `vector_build_info` (modules/logging/_vector.nix gates the Prometheus
# sink on the address alone), so that metric is the fleet's heartbeat.
#
# The awkward part of alerting on absence is that a dead target's series stops existing,
# and a query cannot group by a label that isn't there. `absent_over_time` answers it only
# for a name you spell out — which would mean one rule per target and a hand-kept roster
# of the fleet inside an alert file.
#
# So invert it: ask the data who is missing.
#
#   count by (host) (last_over_time(vector_build_info[24h]))   every target seen in a day
#   unless
#   count by (host) (last_over_time(vector_build_info[10m]))   every target seen recently
#
# `last_over_time` holds a vanished series up for the width of its window, so the left
# side keeps naming a target long after it dies. `unless` is set difference on the label
# set, leaving exactly the targets that were reporting and have gone quiet — one alert
# instance each, `host` label intact. No roster, no registry, nothing to keep in sync: a
# container added tomorrow is covered the first time it reports, and one removed ages out
# of the 24h window on its own.
#
# prometheus is the one target this cannot see, and not by omission: it does not
# remote_write to itself, so it has no heartbeat that could go absent.
#
# The two physical hosts are excluded (host!~"desktop|server"), and this rule only
# watches the containers:
#   desktop  a workstation, powered off every night — a clean shutdown is not a fault,
#            and there is no metric-only way to tell it from a crash. Alerting on it is
#            pure nightly noise.
#   server   hosts this whole pipeline (prometheus, grafana, HA, matrix). If it drops,
#            the rule can neither evaluate nor deliver — the alert could never fire
#            usefully. A container dying while server stays up still does.
#
# noDataState=OK: the healthy fleet produces an empty vector here, so silence is health.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "target_down";
      title = "Target down";
      expr = ''
        count by (host) (last_over_time(vector_build_info{host!~"desktop|server"}[24h]))
          unless
        count by (host) (last_over_time(vector_build_info{host!~"desktop|server"}[10m]))
      '';
      op = "gt";
      value = 0;
      lookback = 86400;
      for = "5m";
      severity = "critical";
      category = "host";
      noDataState = "OK";
      summary = "{{ $labels.host }} stopped reporting";
      description = "{{ $labels.host }} has published no Vector metrics for 10m — the host or container, or its Vector sidecar, is down.";
    })
  ];
}
