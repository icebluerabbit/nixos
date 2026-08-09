# radarr — host configuration override for radarr container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.radarr = {
    aspect-name = "hosts.server.containers.radarr";
    includes = [
      den.aspects.containers._.radarr
      den.aspects.infra._.clients._.container-radarr
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
      { config, lib, ... }:
      let
        cfg = config.containerConfigs.radarr;
        # Assemble a configuratarr list section from one-file-per-entry. Each entry
        # file is a `{ config, ... }:` function returning a single attrset; files are
        # `_`-prefixed so import-tree skips them as modules but readDir still sees them.
        importDir =
          dir:
          map (n: import (dir + "/${n}") { inherit config; }) (
            builtins.filter (lib.hasSuffix ".nix") (builtins.attrNames (builtins.readDir dir))
          );
      in
      {
        containerConfigs.radarr = {

          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-radarr.mac;
          localAddress = "${config.infra.clients.container-radarr.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-radarr.ports.default;
          dbHost = config.infra.clients.container-postgres.ip;
          dbPort = config.infra.clients.container-postgres.ports.default;
          configuratarr = {
            enable = true;
            waitForHealthy = true;
            settings.radarr = {
              type = "radarr-v3";
              url = "http://localhost:${toString config.infra.clients.container-radarr.ports.default}";
              api_key = "\${env.RADARR__AUTH__APIKEY}";
              root_folders = [ { path = "/mnt/media/library/movies"; } ];
              # Enforce hardlink imports (complete/ and library/ share the /mnt/media fs).
              # Singleton is presence-masked: only this field is set, others stay live.
              media_management = {
                copy_using_hardlinks = true;
              };
              # Import lists are additive-only: never unmonitor/remove a movie when it
              # drops off a Trakt (or any) list. Pins Radarr's List Sync Level so a UI
              # click or DB reseed can't silently start cleaning the library.
              import_list_config.list_sync_level = "disabled";
              # List-valued sections are exploded one-file-per-entry under the matching
              # subdir; importDir globs each. Add an entry = drop a file, no edit here.
              custom_formats = importDir ./custom-formats;
              quality_profiles = importDir ./quality-profiles;
              download_clients = importDir ./download-clients;
              import_lists = importDir ./import-lists;
              notifications = importDir ./notifications;
              tags = [
                { label = "watchlist"; }
                { label = "imdb-top-rated"; }
                { label = "trakt-trending"; }
                { label = "80s-horror"; }
                { label = "fangoria-horror"; }
                { label = "rt-horror"; }
                { label = "hidden-horror"; }
                { label = "greatest-sci-fi"; }
                { label = "rt-sci-fi"; }
                { label = "dragonball"; }
                { label = "scifi-cult-classics"; }
                { label = "space-operas"; }
              ];
            };
          };
          exportarr = {
            enable = true;
            port = config.infra.clients.container-radarr.ports.exporter;
          };
          bindMounts = {
            "/var/lib/radarr" = {
              hostPath = "/var/lib/appdata/radarr";
              isReadOnly = false;
            };
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
          };
          envFile = config.sops.templates."radarr-env".path;
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
              "radarr_exporter"
              "radarr_rss_metrics"
              "radarr_import_metrics"
              "radarr_warn_metrics"
            ];
            extraSettings = {
              # radarr's Webhook connection POSTs here on loopback; the sidecar
              # republishes verbatim to MQTT so HA consumes via an mqtt trigger
              # instead of a direct webhook — no notify-time dependency on HA.
              sources.webhook_in = {
                type = "http_server";
                address = "127.0.0.1:9001";
                method = "POST";
                path = "/";
                decoding.codec = "bytes"; # keep radarr's JSON body verbatim → .message
                framing.method = "bytes"; # one event per request body, not per line
              };
              sinks.webhook_mqtt = {
                type = "mqtt";
                inputs = [ "webhook_in" ];
                host = config.infra.clients.container-mosquitto.ip;
                port = config.infra.clients.container-mosquitto.ports.default;
                client_id = "vector-radarr"; # unique per container → no broker client-id collision
                topic = "homelab/notify/radarr";
                quality_of_service = "atleastonce";
                retain = false;
                encoding.codec = "text"; # emit .message raw (the radarr JSON)
              };
              # exportarr sidecar → Radarr state metrics (queue, disk, counts, quality).
              sources.radarr_exporter = {
                type = "prometheus_scrape";
                endpoints = [
                  "http://127.0.0.1:${toString config.infra.clients.container-radarr.ports.exporter}/metrics"
                ];
                scrape_interval_secs = 30;
                # exportarr walks the whole Radarr API per scrape; default 5s timeout
                # is too tight and the scrape fails → stale metrics.
                scrape_timeout_secs = 30;
              };
              # VRL extras — event metrics derived from the journal that the exporter
              # doesn't expose (RSS throughput, imports, warnings).
              transforms = {
                radarr_rss_filter = {
                  type = "filter";
                  inputs = [ "journal" ];
                  condition = ''match(to_string(.message) ?? "", r'RSS Sync Completed')'';
                };
                radarr_rss_parse = {
                  type = "remap";
                  inputs = [ "radarr_rss_filter" ];
                  source = ''
                    msg = to_string(.message) ?? ""
                    m = parse_regex!(msg, r'Reports found: (?P<found>[0-9]+), Reports grabbed: (?P<grabbed>[0-9]+)')
                    .rss_found = to_int!(m.found)
                    .rss_grabbed = to_int!(m.grabbed)
                  '';
                };
                radarr_rss_metrics = {
                  type = "log_to_metric";
                  inputs = [ "radarr_rss_parse" ];
                  metrics = [
                    {
                      type = "gauge";
                      field = "rss_found";
                      name = "radarr_rss_reports_found";
                    }
                    {
                      type = "gauge";
                      field = "rss_grabbed";
                      name = "radarr_rss_reports_grabbed";
                    }
                  ];
                };
                radarr_import_filter = {
                  type = "filter";
                  inputs = [ "journal" ];
                  condition = ''match(to_string(.message) ?? "", r'Assigning file \[.*\] to movie')'';
                };
                radarr_import_metrics = {
                  type = "log_to_metric";
                  inputs = [ "radarr_import_filter" ];
                  metrics = [
                    {
                      type = "counter";
                      field = "message";
                      name = "radarr_imports_total";
                    }
                  ];
                };
                radarr_warn_filter = {
                  type = "filter";
                  inputs = [ "journal" ];
                  condition = ''match(to_string(.message) ?? "", r'\[Warn\]|\[Error\]')'';
                };
                radarr_warn_metrics = {
                  type = "log_to_metric";
                  inputs = [ "radarr_warn_filter" ];
                  metrics = [
                    {
                      type = "counter";
                      field = "message";
                      name = "radarr_warnings_total";
                    }
                  ];
                };
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/radarr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/radarr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@radarr" = {
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
