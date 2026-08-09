# [BitTorrent] session tuning — high-capacity seedbox:
# 96GB RAM, 32-thread CPU, 100TB NFS over 20GbE, 2GbE WAN, private trackers.
{
  defaultSavePath = "/mnt/media/download";
  # incomplete downloads stay local: a torrent in flight is thousands of small
  # out-of-order writes, and on NFS each one is a round trip (measured 7.7k write
  # ops/s, enough to stall every other reader of /mnt/media). qbt moves the finished
  # torrent to defaultSavePath itself, as one sequential copy.
  tempPath = "/var/tmp/qbittorrent";
  tempPathEnabled = true;
  finishedTorrentExportDirectory = "/mnt/media/torrents";
  disableAutoTMMByDefault = false;
  port = 6881;

  diskIOType = "SimplePreadPwrite";
  asyncIOThreadsCount = 128;
  hashingThreadsCount = 16;
  # 20k open-fd pool: NFS open()/close() costs a round-trip each; seeding a
  # large library churns fds against a small cap → constant reopens. Keep more
  # files resident (NOFILE=1048576 covers it).
  filePoolSize = 20000;
  checkingMemUsageSize = 1024;
  diskQueueSize = 67108864;
  coalesceReadWrite = true;
  # FixedSlots (not RateBased): RateBased gates unchoke slots on measured
  # upload rate, so a cold/idle pure seed (0 rate) never opens a slot →
  # leechers stay choked forever → 0 upload (verified: RateBased never sent
  # unchoke; FixedSlots unchoked + served a piece immediately).
  chokingAlgorithm = "FixedSlots";
  # FastestUpload seed choker + a finite maxUploads: concentrate the upload
  # slots on the fastest-pulling peers and choke the slow tail. Measured a
  # grab: ~280 concurrent uploaders at ~110 KB/s each = ~280 random readers on
  # the HDD-backed NFS library for ~30 MB/s aggregate (BDP-limited, well under
  # 2GbE). The slow half contributes little upload but a full share of the
  # seeks. Default RoundRobin rotates slots across ALL peers = max read spread.
  seedChokingAlgorithm = "FastestUpload";
  queueingSystemEnabled = false;

  # connections / bandwidth — fill 2GbE, sized for HIGH-BDP peers.
  # At ~200ms peer RTT a 5MiB send watermark capped per-peer upload at
  # 5MB/0.2s ≈ 25 MB/s — the observed aggregate wall. 32MiB watermark +
  # pinned 8MiB socket buffers fill high-latency pipes; low watermark raised
  # so the buffer refills from disk before it drains. maxUploads = 150 (~half
  # the ~280 concurrent uploaders measured during a grab): with FastestUpload
  # this keeps the fast/productive peers and chokes the slow bottom half, ~halving
  # concurrent random reads on the array while barely touching total upload (the
  # dropped peers were the slowest). An earlier finite cap of 16 tanked upload —
  # but that was under RoundRobin, which wasted the slots on slow peers; with
  # FastestUpload the finite slots land on the fast peers. maxUploadsPerTorrent
  # unlimited so the hottest torrents can soak the whole pool (best ratio).
  # Tune: upload sags well below ~30 MB/s → raise toward 200; seeks still
  # saturate → drop toward 100.
  maxConnections = 5000;
  maxConnectionsPerTorrent = 200;
  maxUploads = 100;
  maxUploadsPerTorrent = -1;
  socketSendBufferSize = 8388608;
  socketReceiveBufferSize = 8388608;
  sendBufferWatermark = 32768;
  sendBufferLowWatermark = 4096;
  # scales per-peer send buffer by that peer's rate (buffer = rate × factor%,
  # capped at the watermark). 300% gives fast leechers fatter buffers → more
  # in-flight → higher per-peer upload; idle peers stay small.
  sendBufferWatermarkFactor = 300;
  connectionSpeed = 300;
  socketBacklogSize = 1000;

  # protocol — private trackers only: no DHT/PeX/LSD
  dht = false;
  pex = false;
  lsd = false;
  encryption = 0;
  # Both (TCP + uTP), monitored trial. uTP widens seeding reach — some leechers
  # (certain NATs, uTP-only clients) are reachable ONLY over uTP, and TCP-only made
  # us invisible to them. The known risk is LEDBAT: uTP's delay-based congestion
  # control can misread VPN-tunnel jitter as congestion and back off, sagging upload
  # RATE. qbt's default mixed-mode prefers TCP when both compete, so TCP (immune to
  # the false signal) still dominates shared peers; uTP only adds the peers TCP can't
  # reach. Outbound uTP is already safe here — qbt-routing clamps the vpnN path MTU to
  # 1420 so uTP packets aren't blackholed. Watch conn_utp / incoming / Upload-by-
  # Transport panels; revert to "TCP" if rate sags more than reach gains.
  btProtocol = "Both";
  # announce to every tracker in a tier at once (vs first-working). Near-zero effect on
  # single-tracker private torrents; helps only multi-tracker entries find more peers.
  announceToAllTrackers = true;
  # true now that tempPath is local — allocates the file up front instead of writing
  # a sparse one piece by piece. Was false because that is the right call on NFS.
  preallocation = true;
  torrentContentLayout = "Original";
}
