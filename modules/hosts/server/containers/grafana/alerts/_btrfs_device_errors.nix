# btrfs device error counters climbing = the disk is failing, right now. This is the
# only storage-health signal the fleet has: there is no SMART exporter anywhere, so
# without this a dying NVMe is invisible until data goes with it.
#
# Any increase at all is the alert — these counters sit at zero forever on healthy
# hardware, so there is no threshold to tune.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "btrfs_device_errors";
      title = "btrfs device errors";
      expr = "increase(node_btrfs_device_errors_total[1h])";
      op = "gt";
      value = 0;
      lookback = 7200;
      for = "5m";
      severity = "critical";
      category = "storage";
      noDataState = "OK";
      summary = "btrfs errors on {{ $labels.host }} ({{ $labels.device }})";
      description = "btrfs reported {{ $labels.type }} errors on {{ $labels.device }} ({{ $labels.host }}) in the last hour. These counters are zero on healthy hardware — treat as a failing disk.";
    })
  ];
}
