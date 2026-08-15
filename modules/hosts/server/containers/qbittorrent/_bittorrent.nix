# [BitTorrent] session tuning — seedbox: 96GB RAM, 32-thread CPU,
# 100TB HDD-backed NFS over 20GbE, 2GbE WAN, private trackers.
#
# Baseline measured 2026-08-15 over 7d (qbittorrent exporter + node PSI):
#   1969 torrents; upload avg 13.3 MB/s peak 110 MB/s; download avg 14.6 MB/s
#   upload slots in use: avg 62, max 102 — pinned at the cap 0.55% of the week
#   peers interested-but-choked: avg 2.9, max 224
#   disk job queue time: avg 4.4k, p95 7.5k, max 15.3k (alert trips at 100k)
#   libtorrent buffers: peak 225 MB
#   host IO full-stall: mean 12.4%, above 20% in 17.6% of 5m windows
#
# Stall correlates only weakly with qbt: in stalled vs quiet windows, slots run
# 66 vs 52 and upload 17.0 vs 10.0 MB/s; at upload above 50 MB/s mean stall is
# 0.166 against a 0.124 overall mean. Seeding concurrency is not the main
# driver of the array stalls — look elsewhere before capping qbt for them.
{
  defaultSavePath = "/mnt/media/download";
  # incomplete torrents write out of order in small pieces; keep that local and
  # let qbt move the finished torrent to NFS as one sequential copy.
  tempPath = "/var/tmp/qbittorrent";
  tempPathEnabled = true;
  finishedTorrentExportDirectory = "/mnt/media/torrents";
  disableAutoTMMByDefault = false;
  port = 6881;

  diskIOType = "SimplePreadPwrite";
  asyncIOThreadsCount = 128;
  hashingThreadsCount = 16;
  # NFS open()/close() each cost a round trip; keep files resident instead of
  # churning a small pool across 1969 torrents. LimitNOFILE=1048576 covers it.
  filePoolSize = 20000;
  checkingMemUsageSize = 1024;
  diskQueueSize = 67108864;
  coalesceReadWrite = true;
  # piece-picker bias toward pieces in the same file extent. Effect not isolated
  # in any metric here — and the picker is leecher-side, so a pure seed sees
  # little of it. Harmless, unverified.
  pieceExtentAffinity = true;
  # maps to libtorrent suggest_read_cache. libtorrent 2.0 has no dedicated read
  # cache and SimplePreadPwrite tracks none, so expect this inert. Kept only for
  # parity if diskIOType ever moves back to MMap.
  suggestMode = true;
  # FixedSlots, not RateBased: RateBased gates unchoke slots on measured upload
  # rate, so an idle pure seed at 0 rate never opens one.
  chokingAlgorithm = "FixedSlots";
  # Only has an effect while maxUploads is finite — at -1 no peer is ever
  # choked and the seed choker has nothing to rank. The pair is one mechanism.
  seedChokingAlgorithm = "FastestUpload";
  queueingSystemEnabled = false;

  maxConnections = 5000;
  # Raised from 200 on 2026-08-15. Connected-but-choked peers cost no seeks, so
  # this widens the pool FastestUpload draws its slots from. No per-torrent
  # series proves the old value bound — global connections peak at 3091/5000 —
  # so this is a cheap guess, not a measured fix.
  maxConnectionsPerTorrent = 300;
  # Raised from 100 on 2026-08-15. The old cap bound only 0.55% of the week but
  # left 224 peers interested-and-choked at the peak, while the disk job queue
  # never exceeded 15% of its alert threshold and slot count tracked IO stall
  # only weakly (66 vs 52 in stalled vs quiet windows). Finite on purpose — see
  # seedChokingAlgorithm above. Revert if average_time_queue p95 passes 25k or
  # the stall-window fraction passes 25%.
  maxUploads = 200;
  maxUploadsPerTorrent = -1;
  socketSendBufferSize = 8388608;
  socketReceiveBufferSize = 8388608;
  sendBufferWatermark = 32768;
  sendBufferLowWatermark = 4096;
  # per-peer buffer scales with that peer's rate, capped at the watermark. Fast
  # leechers get more in flight; idle peers stay small. Total buffers peaked at
  # 225 MB against 96GB RAM — no memory pressure at this setting.
  sendBufferWatermarkFactor = 300;
  connectionSpeed = 300;
  socketBacklogSize = 1000;
  # several peers behind one NAT would otherwise collapse to one connection.
  multiConnectionsPerIp = true;

  # Minimised, not disabled — it cannot be. libtorrent's compare_disconnect_peer
  # ranks candidates on is_interesting, is_seed and payload DOWNLOAD rate, never
  # upload rate, so turnover can drop a fast peer FastestUpload just promoted.
  # 0 floors at 1 peer per pass (max(n * pct / 100, 1)), so push the gate and the
  # timer instead: churn only at the limit, at most one peer an hour.
  peerTurnover = 0;
  peerTurnoverCutOff = 100;
  peerTurnoverInterval = 3600;

  # private trackers only
  dht = false;
  pex = false;
  lsd = false;
  encryption = 0;
  # uTP carries 28% of connections (37.4 of 133.5 avg peers) — reach TCP alone
  # would not have. Mixed mode prefers TCP where both are available. Revert to
  # "TCP" only if upload rate sags without a matching drop in conn_utp.
  btProtocol = "Both";
  # announce to every tracker in a tier at once. Near-inert on single-tracker
  # private torrents.
  announceToAllTrackers = true;
  # allocate up front rather than writing sparse piece by piece — correct now
  # that tempPath is local, would be wrong on NFS.
  preallocation = true;
  torrentContentLayout = "Original";
}
