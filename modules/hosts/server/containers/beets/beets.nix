# beets — host configuration override for the beets container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.beets = {
    aspect-name = "hosts.server.containers.beets";
    includes = [
      den.aspects.containers._.beets
      den.aspects.infra._.clients._.container-beets
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.media
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.beets;
      in
      {
        containerConfigs.beets = {
          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-beets.mac;
          localAddress = "${config.infra.clients.container-beets.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-beets.ports.default;
          importDir = "/mnt/media/rips";
          importInterval = "10min";
          # beets policy — deep-merged over the aspect's structural defaults
          # (directory / library / plugin list / web port stay container-owned).
          settings = {
            import = {
              # move (not copy) off the inbox into the library — same /mnt/media
              # fs, so it's a rename. quiet auto-applies confident matches;
              # quiet_fallback = skip leaves uncertain albums in the inbox for a
              # manual `beet import` over SSH.
              move = true;
              write = true;
              quiet = true;
              quiet_fallback = "skip";
              incremental = true;
              duplicate_action = "skip";
              # whipper tags every rip with authoritative MusicBrainz IDs before it
              # reaches the inbox, and beets' re-autotag was failing to match and
              # skipping the album. Trust the embedded tags instead of re-matching
              # (equivalent to `beet import -A`). fetchart (via mb_albumid),
              # replaygain, lastgenre, embedart, badfiles etc. still run on import.
              autotag = false;
            };
            # Artist/Album layout Lidarr + Jellyfin expect.
            paths = {
              default = "$albumartist/$album%aunique{}/$track $title";
              singleton = "Non-Album/$artist/$title";
              comp = "Compilations/$album%aunique{}/$track $title";
            };
            match.strong_rec_thresh = 0.1;
            # accurate release metadata for archival rips
            original_date = true;
            per_disc_numbering = true;
            threaded = true;
            chroma.auto = true;
            # big, square, hi-res cover as a sidecar (cover.jpg); embed a smaller copy.
            fetchart = {
              auto = true;
              maxwidth = 2000;
              minwidth = 1000;
              enforce_ratio = true;
              high_resolution = true;
            };
            embedart = {
              auto = true;
              maxwidth = 1200; # keep the full-res cover.jpg; embed a lean copy into tags
            };
            replaygain = {
              backend = "ffmpeg";
              auto = true;
              overwrite = false;
            };
            lastgenre = {
              auto = true;
              count = 4;
              canonical = true;
              prefer_specific = true;
              source = "album";
            };
            scrub.auto = true;
            ftintitle.auto = true;
            # synced (timed) lyrics from LRCLIB etc., embedded + written; own rips, so fine.
            lyrics = {
              auto = true;
              synced = true;
              fallback = "";
            };
            # integrity-check each rip on import (flac -t / mp3val).
            badfiles.check_on_import = true;
            # carry whipper's rip sidecars into the album dir on import (beets
            # core only relocates audio). Honors import.move, so they're moved,
            # not left as litter. .log also catches whippit-summary.log.
            filetote.extensions = [
              ".cue"
              ".log"
              ".m3u"
              ".toc"
            ];
          };
          bindMounts = {
            "/var/lib/beets" = {
              hostPath = "/var/lib/appdata/beets";
              isReadOnly = false;
            };
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
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
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/beets 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/beets 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "d ${cfg.importDir} 0775 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@beets" = {
          after = [
            "remote-fs.target"
            "container-wave-apps.target"
          ];
          unitConfig.RequiresMountsFor = "/mnt/media";
        };
      };
  };
}
