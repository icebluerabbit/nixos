# A filesystem is on track to fill within the week. root_disk_low and media_share_low are
# both static thresholds, which means they fire once the problem is already here; this
# extrapolates the last 6h of free space forward 7 days and fires while there is still
# room to act.
#
# predict_linear is a straight-line fit, so it is noisy by nature: any large delete or
# import bends the trend and a short window would fire on every torrent that lands. 6h of
# history against a 7-day horizon, plus for=1h, filters that down to trends that have
# actually persisted.
#
# host_type="host" keeps the bind-mounted duplicates out — every container reports the
# host's own mounts through its sidecar, so an unfiltered rule fires once per container
# for one filling disk.
#
# /var/cache/fscache is excluded: it's a dedicated cachefilesd drive that self-culls (LRU)
# at 7% free and hard-caps at 3%, so it's designed to grow to ~95% full and sit there. A
# linear "fills in 7 days" projection fires forever on it by design — it never runs out.
#
# btrfs is handled factually instead (btrfs_free_low + btrfs_allocation_low): statfs avail
# is UNRELIABLE on btrfs and prediction misfired on every qbt temp spike. This rule keeps
# statfs+predict only for non-btrfs mounts, where avail is trustworthy.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "disk_fill_predicted";
      title = "Disk fill predicted";
      expr = ''
        predict_linear(
          node_filesystem_avail_bytes{fstype!~"tmpfs|ramfs|overlay|btrfs",mountpoint!="/var/cache/fscache",host_type="host"}[6h],
          7 * 24 * 3600
        )
      '';
      op = "lt";
      value = 0;
      lookback = 21600; # must cover the 6h range selector
      for = "1h";
      severity = "warning";
      category = "storage";
      noDataState = "OK";
      summary = "{{ $labels.mountpoint }} on {{ $labels.host }} fills within 7 days";
      description = "At its rate of growth over the last 6h, {{ $labels.mountpoint }} on {{ $labels.host }} runs out of space inside a week. Still time to prune; there won't be once the static threshold fires.";
    })
  ];
}
