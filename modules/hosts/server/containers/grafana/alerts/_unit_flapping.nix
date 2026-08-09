# A systemd unit is restarting in a loop. systemd_unit_failed cannot see this: a unit
# with Restart=always never reaches the failed state, it just dies and comes back, and
# from the outside the service is "active" every time anything looks. The only trace is
# how often its active state changes.
#
# changes() over the active gauge counts every transition in the window, so a unit that
# has restarted more than 5 times in an hour is crash-looping regardless of what its
# current state says. A clean deploy touches a unit once or twice; 5 is comfortably above
# a rebuild and well below a loop.
#
# No host_type filter here — unlike the PSI and meminfo gauges, systemd units are
# genuinely per-namespace, and a container flapping its own unit is exactly what this
# should catch.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "unit_flapping";
      title = "systemd unit flapping";
      expr = ''changes(node_systemd_unit_state{state="active"}[1h])'';
      op = "gt";
      value = 5;
      lookback = 3900; # must cover the 1h range selector
      for = "0m";
      severity = "warning";
      category = "host";
      noDataState = "OK";
      keepFiringFor = "1h";
      summary = "{{ $labels.name }} is restarting in a loop on {{ $labels.host }}";
      description = "systemd unit {{ $labels.name }} on {{ $labels.host }} has changed state more than 5 times in the last hour. It is crash-looping — Restart= keeps bringing it back, so it never reaches the failed state that systemd_unit_failed watches for.";
    })
  ];
}
