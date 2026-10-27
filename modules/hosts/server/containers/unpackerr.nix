# unpackerr — host configuration override for unpackerr container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.unpackerr = {
    aspect-name = "hosts.server.containers.unpackerr";
    includes = [
      den.aspects.containers._.unpackerr
      den.aspects.infra._.clients._.container-unpackerr
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.clients._.container-managarr
      den.aspects.infra._.networks._.media
      den.aspects.infra._.clients._.container-radarr
      den.aspects.infra._.clients._.container-sonarr
      den.aspects.infra._.clients._.container-lidarr
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      {
        containerConfigs.unpackerr = {
          # Where managarr-unpackerr relays what it sees. The read half works
          # without this; relaying is what makes an event reach managarr in
          # seconds rather than at the next hourly pass.
          managarrUrl = "http://${config.infra.clients.container-managarr.ip}:${toString config.infra.clients.container-managarr.ports.default}";

          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-unpackerr.mac;
          localAddress = "${config.infra.clients.container-unpackerr.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;

          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];

          bindMounts = {
            # Must see the same download paths the *arrs do, to reach the archives.
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
          };

          # api_key omitted here — injected per instance via the env file
          # (UN_RADARR_0_API_KEY, …) so it stays out of the nix store.
          envFile = config.sops.templates."unpackerr-env".path;

          settings = {
            interval = "2m";
            start_delay = "1m";
            parallel = 1;
            radarr = [
              {
                url = "http://${config.infra.clients.container-radarr.ip}:${toString config.infra.clients.container-radarr.ports.default}";
                paths = [ "/mnt/media/complete/movies" ];
                protocols = "torrent";
              }
            ];
            sonarr = [
              {
                url = "http://${config.infra.clients.container-sonarr.ip}:${toString config.infra.clients.container-sonarr.ports.default}";
                paths = [ "/mnt/media/complete/tv" ];
                protocols = "torrent";
              }
            ];
            lidarr = [
              {
                url = "http://${config.infra.clients.container-lidarr.ip}:${toString config.infra.clients.container-lidarr.ports.default}";
                paths = [ "/mnt/media/complete/music" ];
                protocols = "torrent";
              }
            ];
            # managarr recorder. unpackerr is the only thing that knows an
            # archive was extracted, and extraction is exactly where the
            # download-side files stop matching what the *arr imports — the
            # extracted file is a new inode, so no hardlink ever ties the two
            # together. Recording the event is how that gets modelled from a
            # fact rather than re-derived from link counts.
            webhook = [
              {
                url = "http://${config.infra.clients.container-managarr.ip}:${toString config.infra.clients.container-managarr.ports.default}/hook/unpackerr";
                name = "managarr";
                # Every event type, and the full payload rather than the
                # trimmed one — this is a corpus, not a notification.
                silent = false;
              }
            ];
            # Built-in Prometheus metrics endpoint (scraped by the Vector sidecar).
            webserver = {
              metrics = true;
              listen_addr = "0.0.0.0:${toString config.infra.clients.container-unpackerr.ports.metrics}";
            };
          };

          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            prometheusInputs = [
              "unpackerr_metrics"
              "unpackerr_queue_metrics"
              "unpackerr_import_metrics"
              "managarr_unpackerr"
            ];
            # ~1400 identical queue heartbeats/day — the numbers already live in
            # unpackerr_queue_metrics, which reads the raw journal source and so is
            # unaffected by this Loki-side drop.
            journalDropPatterns = [ "\\[Unpackerr\\] Queue: [0-9]+ waiting" ];
            extraSettings = {
              # The managarr-* sidecar in this container. Its collectors sit
              # on the loopback: the framework serves /metrics on a separate
              # address from the contract routes, so the sidecar's own port
              # answers /healthz and /changes but never this.
              sources.managarr_unpackerr = {
                type = "prometheus_scrape";
                endpoints = [ "http://127.0.0.1:9909/metrics" ];
                scrape_interval_secs = 30;
              };
              # unpackerr's native Prometheus endpoint → extract/import/failed counts.
              sources.unpackerr_metrics = {
                type = "prometheus_scrape";
                endpoints = [
                  "http://127.0.0.1:${toString config.infra.clients.container-unpackerr.ports.metrics}/metrics"
                ];
                scrape_interval_secs = 30;
                scrape_timeout_secs = 15;
              };
              # VRL extras — parse the periodic "Queue:" summary line into gauges and
              # count per-title import events from the journal.
              transforms = {
                unpackerr_queue_filter = {
                  type = "filter";
                  inputs = [ "journal" ];
                  condition = ''match(to_string(.message) ?? "", r'\[Unpackerr\] Queue:')'';
                };
                unpackerr_queue_parse = {
                  type = "remap";
                  inputs = [ "unpackerr_queue_filter" ];
                  source = ''
                    msg = to_string(.message) ?? ""
                    m = parse_regex!(msg, r'Queue: (?P<waiting>[0-9]+) waiting, (?P<queued>[0-9]+) queued, (?P<extracting>[0-9]+) extracting, (?P<extracted>[0-9]+) extracted, (?P<imported>[0-9]+) imported, (?P<failed>[0-9]+) failed, (?P<deleted>[0-9]+) deleted')
                    .q_waiting = to_int!(m.waiting)
                    .q_queued = to_int!(m.queued)
                    .q_extracting = to_int!(m.extracting)
                    .q_extracted = to_int!(m.extracted)
                    .q_imported = to_int!(m.imported)
                    .q_failed = to_int!(m.failed)
                    .q_deleted = to_int!(m.deleted)
                  '';
                };
                unpackerr_queue_metrics = {
                  type = "log_to_metric";
                  inputs = [ "unpackerr_queue_parse" ];
                  metrics = [
                    {
                      type = "gauge";
                      field = "q_waiting";
                      name = "unpackerr_queue_waiting";
                    }
                    {
                      type = "gauge";
                      field = "q_queued";
                      name = "unpackerr_queue_queued";
                    }
                    {
                      type = "gauge";
                      field = "q_extracting";
                      name = "unpackerr_queue_extracting";
                    }
                    {
                      type = "gauge";
                      field = "q_extracted";
                      name = "unpackerr_queue_extracted";
                    }
                    {
                      type = "gauge";
                      field = "q_imported";
                      name = "unpackerr_queue_imported";
                    }
                    {
                      type = "gauge";
                      field = "q_failed";
                      name = "unpackerr_queue_failed";
                    }
                    {
                      type = "gauge";
                      field = "q_deleted";
                      name = "unpackerr_queue_deleted";
                    }
                  ];
                };
                unpackerr_import_filter = {
                  type = "filter";
                  inputs = [ "journal" ];
                  condition = ''match(to_string(.message) ?? "", r'Imported: .* \(delete in')'';
                };
                unpackerr_import_metrics = {
                  type = "log_to_metric";
                  inputs = [ "unpackerr_import_filter" ];
                  metrics = [
                    {
                      type = "counter";
                      field = "message";
                      name = "unpackerr_imported_total";
                    }
                  ];
                };
              };
            };
          };
        };

        systemd.services."container@unpackerr" = {
          before = [ "container-wave-downstream.target" ];
          wants = [
            "container@radarr.service"
            "container@sonarr.service"
            "container@lidarr.service"
          ];
          after = [
            "container@radarr.service"
            "container@sonarr.service"
            "container@lidarr.service"
            "remote-fs.target"
            "container-wave-arr.target"
          ];
        };
      };
  };
}
