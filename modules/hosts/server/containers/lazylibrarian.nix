# lazylibrarian — host configuration override for lazylibrarian container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.lazylibrarian = {
    aspect-name = "hosts.server.containers.lazylibrarian";
    includes = [
      den.aspects.containers._.lazylibrarian
      den.aspects.infra._.clients._.container-lazylibrarian
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.infra._.clients._.container-prowlarr
      den.aspects.infra._.clients._.container-mosquitto
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.media
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        cfg = config.containerConfigs.lazylibrarian;
        # LazyLibrarian's only structured notify hook. It invokes the script as
        # `script key1 val1 key2 val2 …` with the book's whole DB row (BookID,
        # BookName, AuthorName, Status, Source, Event, …) — the notify_* fan-out
        # that feeds Slack/Apprise only gets a flat prose string with no BookID.
        # Rebuild it as JSON and POST to the Vector sidecar, which republishes to
        # MQTT. Container shares the host /nix/store, so this path resolves inside.
        # LazyLibrarian owns argv — it appends the row's key/value pairs to
        # whatever path it is given — so the binary's own options cannot be set
        # by an ExecStart. This wrapper supplies them ahead of `--` and forwards
        # the rest. Container shares the host /nix/store, so both paths resolve
        # inside.
        notifyScript = pkgs.writeShellScript "lazylibrarian-notify" ''
          exec ${lib.getExe pkgs.homelab.lazylibrarian-notify} \
            --db /var/lib/lazylibrarian/lazylibrarian.db \
            --endpoint http://127.0.0.1:9001/ \
            -- "$@"
        '';
      in
      {
        containerConfigs.lazylibrarian = {

          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-lazylibrarian.mac;
          localAddress = "${config.infra.clients.container-lazylibrarian.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-lazylibrarian.ports.default;
          bindMounts = {
            "/var/lib/lazylibrarian" = {
              hostPath = "/var/lib/appdata/lazylibrarian";
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
          envFile = config.sops.templates."lazylibrarian-env".path;
          configuratarr = {
            enable = true;
            settings.lazylibrarian = {
              type = "lazylibrarian-v1";
              url = "http://127.0.0.1:${toString config.infra.clients.container-lazylibrarian.ports.default}";
              api_key = "\${env.LAZYLIBRARIAN_API_KEY}";
              config = {
                # Keys here are configuratarr's *field* names, not LazyLibrarian's
                # wire names — e.g. `enabled` → API_ENABLED, `script` → CUSTOM_SCRIPT.
                # A wire name silently no-ops: the key is dropped, sync reports success.
                api = {
                  enabled = true;
                  book_api = "OpenLibrary"; # primary metadata source
                  ol_api = true;
                  hc_api = true; # token is per-user in DB — set in UI via /user_admin
                  hc_sync = true;
                  hc_syncreadonly = true; # HardCover → LL only; never write to the HC account
                  hc_sync_limit = 10; # block a sync pushing >10 changes
                };
                torrent.tor_downloader_qbittorrent = true;
                qbittorrent = {
                  host = config.infra.clients.container-qbittorrent.ip;
                  port = config.infra.clients.container-qbittorrent.ports.webui;
                  label = "books"; # → /mnt/media/complete/books (qbt category)
                };
                general = {
                  ebook_dir = "/mnt/media/library/cwa-ingest"; # LL library dest → CWA ingest
                  download_dir = "/mnt/media/complete/books"; # qbt "books" category output
                };
                # Fires on snatch + import only; LL emits no other custom events.
                custom = {
                  use_custom = true;
                  notify_onsnatch = true;
                  notify_ondownload = true;
                  script = "${notifyScript}";
                };
              };
              torznab = [
                {
                  dispname = "PreToMe";
                  # Disabled in prowlarr (indexers/_pretome.nix, `enable = false`), and a
                  # disabled indexer's Torznab feed answers 410 Gone — LL then blocked the
                  # provider for an hour on every search sweep. Flip back with prowlarr.
                  enabled = false;
                  host = "http://${config.infra.clients.container-prowlarr.ip}:${toString config.infra.clients.container-prowlarr.ports.default}/1/api";
                  api = "\${env.PROWLARR_API_KEY}";
                  book_cat = "7000,7020";
                  dl_types = "E";
                }
                {
                  dispname = "TorrentLeech";
                  enabled = true;
                  host = "http://${config.infra.clients.container-prowlarr.ip}:${toString config.infra.clients.container-prowlarr.ports.default}/2/api";
                  api = "\${env.PROWLARR_API_KEY}";
                  book_cat = "7000,7020";
                  comic_cat = "7030";
                  dl_types = "E,C";
                }
              ];
            };
          };
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            # No exportarr for LazyLibrarian (upstream supports the *arrs only, and LL
            # exposes no /metrics), so every metric here is derived from the journal.
            prometheusInputs = [
              "lazylibrarian_search_metrics"
              "lazylibrarian_import_metrics"
              "lazylibrarian_postprocess_metrics"
              "lazylibrarian_warn_metrics"
            ];
            extraSettings = {
              # notifyScript POSTs here on loopback; the sidecar republishes verbatim
              # to MQTT so HA consumes via an mqtt trigger — no notify-time dependency
              # on HA being up. HA then routes onward to Matrix.
              sources.webhook_in = {
                type = "http_server";
                address = "127.0.0.1:9001";
                method = "POST";
                path = "/";
                decoding.codec = "bytes"; # keep the notify JSON body verbatim → .message
                framing.method = "bytes"; # one event per request body, not per line
              };
              sinks.webhook_mqtt = {
                type = "mqtt";
                inputs = [ "webhook_in" ];
                host = config.infra.clients.container-mosquitto.ip;
                port = config.infra.clients.container-mosquitto.ports.default;
                client_id = "vector-lazylibrarian"; # unique per container → no broker collision
                topic = "homelab/notify/lazylibrarian";
                quality_of_service = "atleastonce";
                retain = false;
                encoding.codec = "text"; # emit .message raw (the notify JSON)
              };
              transforms = {
                # "Search for Wanted items complete, found 3 books"
                lazylibrarian_search_filter = {
                  type = "filter";
                  inputs = [ "journal" ];
                  condition = ''match(to_string(.message) ?? "", r'Search for Wanted items complete')'';
                };
                lazylibrarian_search_parse = {
                  type = "remap";
                  inputs = [ "lazylibrarian_search_filter" ];
                  source = ''
                    msg = to_string(.message) ?? ""
                    m = parse_regex!(msg, r'found (?P<found>[0-9]+) book')
                    .books_found = to_int!(m.found)
                  '';
                };
                lazylibrarian_search_metrics = {
                  type = "log_to_metric";
                  inputs = [ "lazylibrarian_search_parse" ];
                  metrics = [
                    {
                      type = "gauge";
                      field = "books_found";
                      name = "lazylibrarian_search_books_found";
                    }
                  ];
                };
                # "Successfully processed: <name>"
                lazylibrarian_import_filter = {
                  type = "filter";
                  inputs = [ "journal" ];
                  condition = ''match(to_string(.message) ?? "", r'Successfully processed:')'';
                };
                lazylibrarian_import_metrics = {
                  type = "log_to_metric";
                  inputs = [ "lazylibrarian_import_filter" ];
                  metrics = [
                    {
                      type = "counter";
                      field = "message";
                      name = "lazylibrarian_imports_total";
                    }
                  ];
                };
                # "4 downloads processed."
                lazylibrarian_postprocess_filter = {
                  type = "filter";
                  inputs = [ "journal" ];
                  condition = ''match(to_string(.message) ?? "", r'[0-9]+ downloads? processed\.')'';
                };
                lazylibrarian_postprocess_parse = {
                  type = "remap";
                  inputs = [ "lazylibrarian_postprocess_filter" ];
                  source = ''
                    msg = to_string(.message) ?? ""
                    m = parse_regex!(msg, r'(?P<count>[0-9]+) downloads? processed')
                    .processed = to_int!(m.count)
                  '';
                };
                lazylibrarian_postprocess_metrics = {
                  type = "log_to_metric";
                  inputs = [ "lazylibrarian_postprocess_parse" ];
                  metrics = [
                    {
                      type = "gauge";
                      field = "processed";
                      name = "lazylibrarian_downloads_processed";
                    }
                  ];
                };
                # LL's python logging format is "<ts> WARNING: …" / "<ts> ERROR: …",
                # not the *arrs' "[Warn]"/"[Error]".
                #
                # Upstream bug: api.py `_listdirectproviders` reads BOK_LOGIN/BOK_USER,
                # which configdefs never defines (the keys are BOK_EMAIL/BOK_REMIX_USERID),
                # so every configuratarr sync logs 4 spurious ERRORs. Exclude them, or this
                # counter measures rebuild frequency rather than service health.
                lazylibrarian_warn_filter = {
                  type = "filter";
                  inputs = [ "journal" ];
                  condition = ''
                    msg = to_string(.message) ?? ""
                    match(msg, r'\b(WARNING|ERROR):') && !match(msg, r'Config\[BOK_(LOGIN|USER)\]')
                  '';
                };
                lazylibrarian_warn_metrics = {
                  type = "log_to_metric";
                  inputs = [ "lazylibrarian_warn_filter" ];
                  metrics = [
                    {
                      type = "counter";
                      field = "message";
                      name = "lazylibrarian_warnings_total";
                    }
                  ];
                };
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/lazylibrarian 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/lazylibrarian 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@lazylibrarian" = {
          before = [ "container-wave-downstream.target" ];
          after = [
            "remote-fs.target"
            "container-wave-arr.target"
          ];
        };
      };
  };
}
