# Btrfs unallocated space is the real "disk full" signal — not statfs free space. Btrfs
# can only create new data/metadata chunks out of *unallocated* device space; once that
# hits zero it throws ENOSPC even when df shows room, because the free space is stranded
# inside already-allocated chunks of the wrong type. This is the classic btrfs footgun,
# and the one btrfs-balance exists to prevent.
#
# btrfs_free_low (below) fires earlier at <10% true free for capacity planning; this is the
# hard backstop that fires when unallocated actually falls into the ENOSPC danger zone.
# Aggregated per pool (uuid) so one alert per filesystem, not per device.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "btrfs_allocation_low";
      title = "Btrfs unallocated space low";
      expr = "sum by (uuid, host) (node_btrfs_device_unused_bytes{host_type=\"host\"})";
      op = "lt";
      value = 21474836480; # 20 GiB
      lookback = 300;
      for = "30m";
      severity = "warning";
      category = "storage";
      noDataState = "OK";
      summary = "btrfs unallocated space low on {{ $labels.host }}";
      description = "The btrfs pool ({{ $labels.uuid }}) on {{ $labels.host }} has under 20 GiB of unallocated space left. Btrfs allocates new chunks from unallocated space only — at zero it hits ENOSPC even with free space showing in df. Run a balance (btrfs-balance) or free real data now.";
    })
    (promRule {
      uid = "btrfs_free_low";
      title = "Btrfs pool nearly full";
      # Factual true-free ratio: (unallocated + slack inside allocated data chunks) / raw
      # pool size. statfs (df) is unreliable on btrfs, so compute it from the btrfs collector.
      expr = ''
        (
          sum by (uuid, host) (node_btrfs_device_unused_bytes{host_type="host"})
          + sum by (uuid, host) (
              node_btrfs_size_bytes{block_group_type="data",host_type="host"}
              - node_btrfs_used_bytes{block_group_type="data",host_type="host"}
            )
        )
        / sum by (uuid, host) (node_btrfs_device_size_bytes{host_type="host"})
      '';
      op = "lt";
      value = 0.1;
      lookback = 300;
      for = "30m";
      severity = "warning";
      category = "storage";
      noDataState = "OK";
      summary = "btrfs pool on {{ $labels.host }} under 10% free";
      description = "The btrfs pool ({{ $labels.uuid }}) on {{ $labels.host }} has under 10% real free space (unallocated + data-chunk slack, not the misleading df figure). Free real data or rebalance.";
    })
  ];
}
