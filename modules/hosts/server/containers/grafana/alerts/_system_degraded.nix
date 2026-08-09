# systemd's own summary verdict for the machine (`systemctl is-system-running`).
# Redundant with systemd_unit_failed by construction, but it is the one number that
# says "this host is not fully up" without enumerating units — and it catches degraded
# states that never surface as a single failed unit.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "system_degraded";
      title = "System degraded";
      expr = "node_systemd_system_running";
      op = "lt";
      value = 1;
      for = "10m";
      severity = "warning";
      category = "host";
      noDataState = "OK";
      summary = "{{ $labels.host }} is degraded";
      description = "systemd on {{ $labels.host }} reports the system is not fully running (degraded/maintenance).";
    })
  ];
}
