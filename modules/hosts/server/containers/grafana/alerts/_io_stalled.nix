# The host is stalled waiting on IO. This is PSI (pressure stall information), not
# utilisation: node_pressure_io_stalled_seconds_total counts wall-clock seconds during
# which *every* runnable task was blocked on IO, so a rate of 0.2 means the machine spent
# 20% of that period doing nothing at all but waiting for a disk or an NFS server.
#
# Load average cannot express this — a host blocked on a hung NFS mount shows a load of
# 100 with an idle CPU, and a host doing real work shows the same number. PSI separates
# "busy" from "stuck", and only the second one is an alert.
#
# host_type="host": PSI is a kernel-wide gauge, reported identically by every container
# sidecar on the same kernel. Without the filter one stall alerts once per container.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "io_stalled";
      title = "IO stalled";
      # PSI can't tell "qBittorrent seeding" (expected, nothing to do — random 16KB
      # reads off the HDD-backed UNAS that no client-side cache can absorb) from "the
      # NFS mount has actually stopped answering" (real, every service is down). Both
      # peg io-stall at 90%. Two signals separate them: a seed storm drives a high NFS
      # READ-op rate whose reads *complete* (slow, no timeouts); a dead mount *throws*
      # RPC major_timeouts. So the benign signature is "NFS busy with reads AND not
      # timing out".
      #
      # Fire on io-stall, `unless` the host matches that benign signature. Gating on the
      # READ rate (not merely "has an NFS mount" — both hosts mount /mnt/media) keeps
      # the alert's other half alive: a LOCAL disk saturating with NFS idle has a low
      # read rate, so it is never suppressed. `on(host)` matches purely on host:
      #   - server mid-seed-storm (reads ≫200, no timeouts) → suppressed → silent
      #   - server, mount hung (major_timeouts>0)           → not suppressed → fires
      #   - any host, local disk saturated, NFS idle        → low reads → fires
      # 200 READ ops/s: today's storms floored at ~840; idle NFS sits under ~170.
      expr = ''
        rate(node_pressure_io_stalled_seconds_total{host_type="host"}[5m])
          unless on (host)
          (
            (sum by (host) (rate(node_mountstats_nfs_operations_requests_total{operation="READ"}[5m])) > 200)
            unless on (host)
            (sum by (host) (rate(node_mountstats_nfs_operations_major_timeouts_total[5m])) > 0)
          )
      '';
      op = "gt";
      value = 0.2;
      lookback = 900;
      # 45m (was 15m): even with the timeout gate, require a sustained stall before
      # paging — a genuinely hung mount stays stalled for far longer than any transient.
      for = "45m";
      severity = "warning";
      category = "host";
      noDataState = "OK";
      keepFiringFor = "15m";
      summary = "{{ $labels.host }} is stalled on IO";
      description = "{{ $labels.host }} has spent more than 20% of the last 15m with every task blocked on IO. Either a local disk is saturated or a network mount has stopped answering — every service on this host is slow, whatever their own metrics say.{{ if eq $labels.host \"server\" }} Usual cause: qBittorrent seeding random 16KB reads off the HDD-backed UNAS (NFS /mnt/media); the fscache SSD (nvme1n1) can't absorb it — re-read locality ~0, ~2% hit. Pin it fast: per-cgroup io.pressure (qbt cgroup ~95% = it's qbt). Also check nvme1n1 util + NFS READ ops/s. Do NOT cap qbt — HDD IOPS wall, not rates.{{ end }}";
    })
  ];
}
