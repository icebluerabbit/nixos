# NFS RPCs are being retransmitted. The lifetime counter on this fleet is 1, so a
# sustained rate is unambiguous: either the transport is dropping or the NAS is not
# answering in time. Everything on /mnt/media stalls behind it.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "nfs_retransmissions";
      title = "NFS retransmissions";
      expr = ''rate(node_nfs_rpc_retransmissions_total{host_type="host"}[5m])'';
      op = "gt";
      value = 0;
      for = "10m";
      severity = "warning";
      category = "storage";
      noDataState = "OK";
      keepFiringFor = "15m";
      summary = "{{ $labels.host }} is retransmitting NFS RPCs";
      description = "NFS RPCs from {{ $labels.host }} are being retransmitted. The mount is not getting timely answers from the NAS — expect every reader of /mnt/media to stall.";
    })
  ];
}
