# The restic backup of /var/lib/appdata has not run. systemd's OnFailure= already
# notifies when a run fails loudly; this covers the silent half — a masked or disabled
# timer, a host that was down at 03:00, a repository that stopped being writable — where
# nothing fails because nothing is attempted.
#
# noDataState is Alerting, and that is the point of the rule. Every other outcome here
# means "the backup is older than it should be"; silence means the exporter that would
# tell you can no longer answer, which is not evidence the backup ran. A backup alert
# that goes quiet when its own telemetry dies is worse than no backup alert.
#
# 36h, not 24h: the timer is daily, so a 24h threshold fires on any run that slips past
# its window (boot delay, a long-running snapshot). Two missed days is unambiguous.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "backup_stale";
      title = "Backup stale";
      expr = ''time() - node_systemd_timer_last_trigger_seconds{name="restic-appdata.timer"}'';
      op = "gt";
      value = 129600; # 36h
      for = "1h";
      severity = "critical";
      category = "storage";
      noDataState = "Alerting";
      missingSeriesEvalsToResolve = 4;
      summary = "restic appdata backup has not run in 36h";
      description = "The daily restic backup of /var/lib/appdata to the NAS last ran more than 36h ago. The timer is masked, the host was down at 03:00, or the repository is no longer writable — either way there is no fresh snapshot.";
    })
  ];
}
