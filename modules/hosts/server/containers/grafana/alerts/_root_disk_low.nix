# Root filesystem filling up on a host. The nix store lives here, so hitting zero
# means no rebuilds, no GC, and containers that cannot start.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "root_disk_low";
      title = "Root disk low";
      expr = ''node_filesystem_avail_bytes{mountpoint="/",fstype!="tmpfs"} / node_filesystem_size_bytes{mountpoint="/",fstype!="tmpfs"}'';
      op = "lt";
      value = 0.1;
      for = "15m";
      severity = "warning";
      category = "storage";
      noDataState = "OK";
      summary = "{{ $labels.host }} root disk below 10% free";
      description = "Root filesystem on {{ $labels.host }} is under 10% free. Run `just gc` or prune generations.";
    })
  ];
}
