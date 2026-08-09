# qBittorrent's own disk-job queue latency — the write path's view from inside the
# client, complementary to host PSI. Baseline sits ~7k; a sustained climb to 100k+
# means the NFS/local write path is backing up before it shows as host IO stall.
{ promRule, ... }:
{
  group = "media";
  rules = [
    (promRule {
      uid = "qbt_disk_queue";
      title = "qBittorrent disk queue backed up";
      expr = ''qbittorrent_global_average_time_queue{host="qbittorrent"}'';
      op = "gt";
      value = 100000;
      for = "15m";
      lookback = 900;
      keepFiringFor = "15m";
      severity = "warning";
      category = "qbittorrent";
      noDataState = "OK";
      summary = "qBittorrent disk queue time is high";
      description = "qBittorrent's average disk-job queue time has stayed above 100k (~14x its ~7k baseline) for 15m. The write path — local incomplete dir or the NFS move — is backing up; expect stalled seeding and slow imports before host IO pressure even registers.";
    })
  ];
}
