# soularr — host configuration override for the soularr container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.soularr = {
    aspect-name = "hosts.server.containers.soularr";
    includes = [
      den.aspects.containers._.soularr
      den.aspects.infra._.clients._.container-soularr
      den.aspects.infra._.clients._.container-lidarr
      den.aspects.infra._.clients._.container-slskd
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.clients._.container-mosquitto
      den.aspects.infra._.networks._.media
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, lib, ... }:
      let
        cfg = config.containerConfigs.soularr;
        exporter = import ./_exporter.nix;
        notify = import ./_notify.nix { inherit config; };
      in
      {
        containerConfigs.soularr = {
          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-soularr.mac;
          localAddress = "${config.infra.clients.container-soularr.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-soularr.ports.webui;

          # endpoints soularr drives (both reachable cross-VLAN, same as Lidarr↔qbittorrent)
          lidarrUrl = "http://${config.infra.clients.container-lidarr.ip}:${toString config.infra.clients.container-lidarr.ports.default}";
          slskdUrl = "http://${config.infra.clients.container-slskd.ip}:${toString config.infra.clients.container-slskd.ports.web}";

          # Paged sweep: each run grabs one page of numberOfAlbumsToGrab missing albums,
          # imports them, exits. OnUnitActiveSec fires this long after the previous run
          # finishes, walking the wanted list a page at a time.
          intervalSeconds = 3600;

          # [Lidarr] — sync the library before searching so wanted state is fresh
          downloadDir = "/mnt/media/complete/soulseek";
          disableSync = false;

          # [Slskd] — auth disabled → key ignored; give stalled/queued grabs generous timeouts
          slskdApiKey = "disabled";
          slskdUrlBase = "/";
          deleteSearches = false;
          stalledTimeout = 3600;
          # match the run interval: don't cancel a remotely-queued download before the next
          # run would re-evaluate it — a peer with full upload slots often serves within a cycle.
          remoteQueueTimeout = cfg.intervalSeconds;

          # [Release Settings] — accept any acceptable release; lossless-friendly formats/regions
          useSelectedLidarrRelease = false;
          useMostCommonTracknum = true;
          allowMultiDisc = true;
          acceptedCountries = "Europe,Japan,United Kingdom,United States,[Worldwide],Australia,Canada";
          skipRegionCheck = false;
          acceptedFormats = "CD,Digital Media,Vinyl";

          # [Search Settings] — prefer hi-res FLAC → FLAC → high-bitrate MP3; skip slow/backed-up peers
          # slskd waits this long for a peer to answer; the wait is what surfaces as
          # "The wait timed out after 5000 milliseconds" on the slskd side (~350/day,
          # each one an HTTP 500 back to soularr plus a Python traceback). Peers behind
          # VPN/queues routinely need longer than 5s. Not a cure — an unreachable peer
          # still times out — but it converts most of the noise into real results.
          searchTimeout = 15000;
          maximumPeerQueue = 20; # drop peers with long queues from results → rotate off busy sharers, spread load
          minimumPeerUploadSpeed = 100000; # ~100 KB/s floor — skip dead-slow peers
          minimumFilenameMatchRatio = "0.9"; # 0.8 let a 0.82 compilation-track false-positive through
          minimumSearchInterval = 10; # space searches 10s apart — daily sweep isn't time-pressed, gentler on Soulseek
          allowedFiletypes = "flac 24/192,flac 16/44.1,flac,mp3 320,mp3";
          albumPrependArtist = false;
          # Paged sweep: each run pulls ONE page of numberOfAlbumsToGrab albums (advancing a
          # persistent .current_page.txt counter, wrapping at the end), enqueues + imports just
          # that page, then exits. Bounds the in-flight download set so a crash/NFS stall strands
          # at most one page — not the whole ~4600-album wanted list (which search_type=all did,
          # causing the mid-sweep lockup and the stranded un-imported backlog).
          searchType = "incrementing_page";
          numberOfAlbumsToGrab = 10; # per-run Soulseek grab cap in paged mode. Small = frequent
          # imports, tiny NFS burst, minimal crash exposure. Raise cautiously.
          # missing only. cutoff_unmet (quality upgrades) re-downloads things you already have —
          # leave off until the backlog is drained and the box is stable.
          searchSource = "missing";
          failedImportDenylist = true;

          # [Download Settings] — strip junk; keep lyrics/nfo/txt alongside audio
          downloadFiltering = true;
          # Keep bundled sidecar extras (lyrics/art/cue/log), not just the audio — soularr
          # strips ALL non-audio when the whitelist is off. Lidarr then imports them.
          useExtensionWhitelist = true;
          extensionsWhitelist = "lrc,cue,log,m3u,jpg,jpeg,png,nfo,txt";

          bindMounts = {
            "/var/lib/soularr" = {
              hostPath = "/var/lib/appdata/soularr";
              isReadOnly = false;
            };
            # slskd writes completes here (all_squash on the share); Lidarr hardlink-imports
            # from the same path. soularr reads it to match/verify downloads.
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
            # raw Lidarr API key (reused sops secret); soularr-config injects it into config.ini.
            "/run/secrets/soularr-lidarr-key" = {
              hostPath = config.sops.secrets."lidarr-apikey".path;
              isReadOnly = true;
            };
          };
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            # journal-derived metrics (exporter) + notify producer, merged into one Vector config
            inherit (exporter) prometheusInputs;
            extraSettings = lib.recursiveUpdate exporter.extraSettings notify.extraSettings;
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/soularr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/soularr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        # Needs Lidarr + slskd reachable, and the media NFS mount for the download dir.
        systemd.services."container@soularr" = {
          after = [
            "remote-fs.target"
            "container@lidarr.service"
            "container@slskd.service"
          ];
          wants = [
            "container@lidarr.service"
            "container@slskd.service"
          ];
        };
      };
  };
}
