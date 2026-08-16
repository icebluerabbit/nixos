# lidarr — host configuration override for lidarr container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.lidarr = {
    aspect-name = "hosts.server.containers.lidarr";
    includes = [
      den.aspects.containers._.lidarr
      den.aspects.infra._.clients._.container-lidarr
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.infra._.clients._.container-prowlarr
      den.aspects.infra._.clients._.container-home-assistant
      den.aspects.infra._.clients._.container-jellyfin
      den.aspects.infra._.clients._.container-mosquitto
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.media
      den.aspects.containers._.postgresql
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.lidarr;
      in
      {
        containerConfigs.lidarr = {

          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-lidarr.mac;
          localAddress = "${config.infra.clients.container-lidarr.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-lidarr.ports.default;
          dbHost = config.infra.clients.container-postgres.ip;
          dbPort = config.infra.clients.container-postgres.ports.default;
          configuratarr = {
            enable = true;
            waitForHealthy = true;
            settings = {
              lidarr = {
                type = "lidarr-v1";
                url = "http://localhost:${toString config.infra.clients.container-lidarr.ports.default}";
                api_key = "\${env.LIDARR__AUTH__APIKEY}";
                root_folders = [
                  {
                    name = "Music";
                    path = "/mnt/media/library/music";
                    default_quality_profile_id = "\${ref.quality_profile.Any}";
                    default_metadata_profile_id = "\${ref.metadata_profile.Standard}";
                    default_monitor_option = "all";
                    default_new_item_monitor_option = "all";
                  }
                ];
                custom_formats = [
                  (import ./custom-formats/_freeleech.nix { inherit config; })
                ];
                # Both profiles are verbatim dumps of Lidarr's built-ins (quality 1 =
                # "Any", metadata 1 = "Standard"), so adopting them changes no grab or
                # monitor behaviour — it only puts them under config, which lets the
                # root folder reference them by name instead of by raw id.
                quality_profiles = [
                  (import ./quality-profiles/_any.nix { inherit config; })
                ];
                metadata_profiles = [
                  (import ./metadata-profiles/_standard.nix { inherit config; })
                ];
                download_clients = [
                  (import ./download-clients/_qbittorrent.nix { inherit config; })
                ];
                # Lidarr has no listSyncLevel/cleanLibraryLevel (unlike radarr/sonarr —
                # verified absent from the Lidarr OpenAPI spec), so there is no list-sync
                # guard to pin here: dropping an artist on Spotify cannot unmonitor or
                # remove them from the library.
                import_lists = [
                  (import ./import-lists/_spotify-followed-artists.nix { inherit config; })
                ];
                # Enforce hardlink imports (complete/ and library/ share the /mnt/media fs).
                # Singleton is presence-masked: only this field is set, others stay live.
                media_management = {
                  copy_using_hardlinks = true;
                  # Import sidecar extras (lyrics/cue/log/art) present in the download folder.
                  # All new files — never modifies audio, so torrent-safe. (Fetched art comes
                  # from the Kodi metadata consumer below, not from the peer.)
                  #
                  # toc/xml are whipper's: the .toc describes the disc layout and the .xml is
                  # the rip's MusicBrainz submission. Without them here, a CD rip coming in
                  # via beets loses half its provenance at import, and the .log — the proof
                  # of rip quality — arrives without the disc data that backs it.
                  import_extra_files = true;
                  extra_file_extensions = "lrc,cue,log,m3u,toc,xml,jpg,jpeg,png,nfo,txt";
                };
                # Album/artist artwork + NFO as sidecar files (cover.jpg, *.nfo). Never rewrites
                # audio tags, so it can't corrupt an actively-seeding hardlinked file.
                metadata = [ (import ./metadata/_kodi.nix { }) ];
                # Organise into Artist/Album (Year)/NN - Title. Was unset → Lidarr's flat
                # default dumped tracks straight in the artist folder (album #2 would collide
                # by track number). rename_tracks=true reorganises existing files on the next
                # rename/refresh (hardlinks within /mnt/media survive — a reshuffle, no
                # re-download). Multi-disc stays one album folder, tracks disc-prefixed.
                # Singleton is presence-masked: only these fields change, the rest stay live.
                naming = {
                  rename_tracks = true;
                  replace_illegal_characters = true;
                  artist_folder_format = "{Artist Name}";
                  standard_track_format = "{Album Title} ({Release Year})/{track:00} - {Track Title}";
                  multi_disc_track_format = "{Album Title} ({Release Year})/{medium:0}-{track:00} - {Track Title}";
                };
                notifications = [
                  (import ./notifications/_jellyfin.nix { inherit config; })
                  (import ./notifications/_home-assistant.nix { inherit config; })
                ];
              };
            };
          };
          exportarr = {
            enable = true;
            port = config.infra.clients.container-lidarr.ports.exporter;
          };
          bindMounts = {
            "/var/lib/lidarr" = {
              hostPath = "/var/lib/appdata/lidarr";
              isReadOnly = false;
            };
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
          };
          envFile = config.sops.templates."lidarr-env".path;
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            prometheusInputs = [
              "lidarr_exporter"
              "lidarr_rss_metrics"
              "lidarr_warn_metrics"
            ];
            extraSettings = {
              # lidarr's Webhook connection POSTs here on loopback; the sidecar
              # republishes verbatim to MQTT so HA consumes via an mqtt trigger
              # instead of a direct webhook — no notify-time dependency on HA.
              sources.webhook_in = {
                type = "http_server";
                address = "127.0.0.1:9001";
                method = "POST";
                path = "/";
                decoding.codec = "bytes"; # keep lidarr's JSON body verbatim → .message
                framing.method = "bytes"; # one event per request body, not per line
              };
              sinks.webhook_mqtt = {
                type = "mqtt";
                inputs = [ "webhook_in" ];
                host = config.infra.clients.container-mosquitto.ip;
                port = config.infra.clients.container-mosquitto.ports.default;
                client_id = "vector-lidarr"; # unique per container → no broker client-id collision
                topic = "homelab/notify/lidarr";
                quality_of_service = "atleastonce";
                retain = false;
                encoding.codec = "text"; # emit .message raw (the lidarr JSON)
              };
              # exportarr sidecar → Lidarr state metrics (artists, albums, songs, queue, health).
              sources.lidarr_exporter = {
                type = "prometheus_scrape";
                endpoints = [
                  "http://127.0.0.1:${toString config.infra.clients.container-lidarr.ports.exporter}/metrics"
                ];
                scrape_interval_secs = 30;
                # exportarr walks the whole Lidarr API per scrape; default 5s timeout
                # is too tight and the scrape fails → stale metrics.
                scrape_timeout_secs = 30;
              };
              # VRL extras — event metrics derived from the journal that the exporter
              # doesn't expose (RSS throughput, warnings).
              transforms = {
                lidarr_rss_filter = {
                  type = "filter";
                  inputs = [ "journal" ];
                  condition = ''match(to_string(.message) ?? "", r'RSS Sync Completed')'';
                };
                lidarr_rss_parse = {
                  type = "remap";
                  inputs = [ "lidarr_rss_filter" ];
                  source = ''
                    msg = to_string(.message) ?? ""
                    m = parse_regex!(msg, r'Reports found: (?P<found>[0-9]+), Reports grabbed: (?P<grabbed>[0-9]+)')
                    .rss_found = to_int!(m.found)
                    .rss_grabbed = to_int!(m.grabbed)
                  '';
                };
                lidarr_rss_metrics = {
                  type = "log_to_metric";
                  inputs = [ "lidarr_rss_parse" ];
                  metrics = [
                    {
                      type = "gauge";
                      field = "rss_found";
                      name = "lidarr_rss_reports_found";
                    }
                    {
                      type = "gauge";
                      field = "rss_grabbed";
                      name = "lidarr_rss_reports_grabbed";
                    }
                  ];
                };
                lidarr_warn_filter = {
                  type = "filter";
                  inputs = [ "journal" ];
                  condition = ''match(to_string(.message) ?? "", r'\[Warn\]|\[Error\]')'';
                };
                lidarr_warn_metrics = {
                  type = "log_to_metric";
                  inputs = [ "lidarr_warn_filter" ];
                  metrics = [
                    {
                      type = "counter";
                      field = "message";
                      name = "lidarr_warnings_total";
                    }
                  ];
                };
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/lidarr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/lidarr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@lidarr" = {
          before = [ "container-wave-arr.target" ];
          wants = [ "container@postgresql.service" ];
          after = [
            "container@postgresql.service"
            "remote-fs.target"
            "container-wave-apps.target"
          ];
        };
      };
  };
}
