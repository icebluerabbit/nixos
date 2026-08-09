# A systemd unit entered `failed` on a host (server/desktop — the only two running the
# systemd exporter). Covers the whole surface no other rule does: timers, one-shots,
# the nspawn container units themselves, sops activation, backups.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "systemd_unit_failed";
      title = "systemd unit failed";
      expr = ''systemd_unit_state{state="failed"}'';
      op = "gt";
      value = 0;
      for = "5m";
      severity = "warning";
      category = "host";
      noDataState = "OK";
      summary = "{{ $labels.name }} failed on {{ $labels.host }}";
      description = "systemd unit {{ $labels.name }} is in the failed state on {{ $labels.host }}.";
    })
  ];
}
