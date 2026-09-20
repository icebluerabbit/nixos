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
          # Where managarr-beets relays what it sees. The read half works
          # without this; relaying is what makes an event reach managarr in
          # seconds rather than at the next hourly pass.
          managarrUrl = "http://${config.infra.clients.container-managarr.ip}:${toString config.infra.clients.container-managarr.ports.default}";
          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-beets.mac;
          localAddress = "${config.infra.clients.container-beets.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-beets.ports.default;
          # whippit promotes finished rips into the watch dir; beets tags them
          # and writes to the complete dir, where they stay. Lidarr links them
          # into the library from there, exactly as it links a finished torrent
          # out of complete/music — a rip reaches the library the same way a
          # download does, and the rip survives as its own tree.
          importDir = "/mnt/media/watch/rips";
          outputDir = "/mnt/media/complete/rips";
          importInterval = "10min";
          lidarrUrl = "http://${config.infra.clients.container-lidarr.ip}:${toString config.infra.clients.container-lidarr.ports.default}";
          # Hardlink, not duplicate: Lidarr's copyUsingHardlinks is on and both
          # trees share /mnt/media, so the retained rip costs no extra space.
          lidarrImportMode = "Copy";
          envFile = config.sops.templates."beets-env".path;
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
            # Every path is exactly <artist>/<album>/<file>, and all three
            # templates must hold that shape. Two things depend on it:
            #
            #   - Nothing is ever left loose in an artist folder. The previous
            #     singleton template was "Non-Album/$artist/$title" — a bare
            #     file with no album directory at all.
            #   - The parent of an album directory is always a real artist
            #     name, because Lidarr identifies an import by parsing that
            #     folder name. "Non-Album" and "Compilations" resolve to no
            #     artist, so those albums would never import and would sit here
            #     unnoticed.
            #
            # Lidarr renames into the library afterwards, so this layout only
            # has to be structurally correct, not pretty.
            paths = {
              default = "$albumartist/$album%aunique{}/$track $title";
              singleton = "$albumartist/Non-Album/$title";
              comp = "Various Artists/$album%aunique{}/$track $title";
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
              ".xml"
              ".nfo"
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
          # Lidarr moves albums out of here, so it needs write access too; the
          # media group is what both containers share.
          "d ${cfg.outputDir} 0775 ${toString cfg.uid} ${toString cfg.gid} - -"
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
