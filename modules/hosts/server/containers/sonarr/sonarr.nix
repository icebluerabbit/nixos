# sonarr — host configuration override for sonarr container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.sonarr = {
    aspect-name = "hosts.server.containers.sonarr";
    includes = [
      den.aspects.containers._.sonarr
      den.aspects.infra._.clients._.container-sonarr
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.infra._.clients._.container-prowlarr
      den.aspects.infra._.clients._.container-home-assistant
      den.aspects.infra._.clients._.container-jellyfin
      den.aspects.infra._.clients._.container-mosquitto
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.clients._.container-managarr
      den.aspects.infra._.networks._.media
      den.aspects.containers._.postgresql
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
        cfg = config.containerConfigs.sonarr;
        # Assemble a configuratarr list section from one-file-per-entry. Each entry
        # file is a `{ config, ... }:` function returning a single attrset; files are
        # `_`-prefixed so import-tree skips them as modules but readDir still sees them.
        # pkgs is the HOST's package set, which is what an entry naming a
        # homelab package needs: the store path is what crosses into the
        # container, and pkgs inside a container body carries no overlays.
        importDir =
          dir:
          map (n: import (dir + "/${n}") { inherit config lib pkgs; }) (
            builtins.filter (lib.hasSuffix ".nix") (builtins.attrNames (builtins.readDir dir))
          );
      in
      {
        containerConfigs.sonarr = {
          # Where managarr-sonarr relays what it sees. The read half works
          # without this; relaying is what makes an event reach managarr in
          # seconds rather than at the next hourly pass.
          managarrUrl = "http://${config.infra.clients.container-managarr.ip}:${toString config.infra.clients.container-managarr.ports.default}";

          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-sonarr.mac;
          localAddress = "${config.infra.clients.container-sonarr.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-sonarr.ports.default;
          dbHost = config.infra.clients.container-postgres.ip;
          dbPort = config.infra.clients.container-postgres.ports.default;
          configuratarr = {
            enable = true;
            waitForHealthy = true;
            settings.sonarr = {
              type = "sonarr-v3";
              url = "http://localhost:${toString config.infra.clients.container-sonarr.ports.default}";
              api_key = "\${env.SONARR__AUTH__APIKEY}";
              root_folders = [ { path = "/mnt/media/library/tv"; } ];
              # Enforce hardlink imports (complete/ and library/ share the /mnt/media fs).
              # Singleton is presence-masked: only this field is set, others stay live.
              media_management = {
                copy_using_hardlinks = true;
              };
              # Anchor the series folder to its TVDB id. Jellyfin parses the
              # [tvdbid-N] tag out of the folder name and skips its own title
              # search, so a short title can't lose to a popular prefix match
              # ("ted" -> Ted Lasso). Singleton is presence-masked: only this
              # field is set, the other naming formats stay live.
              naming = {
                series_folder_format = "{Series TitleYear} [tvdbid-{TvdbId}]";
              };
              # Import lists are additive-only: never unmonitor/remove a series when it
              # drops off a Trakt (or any) list. Pins Sonarr's List Sync Level so a UI
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
                { label = "anime"; }
                { label = "dragonball"; }
                { label = "scifi-tv"; }
                { label = "space-operas"; }
              ];
            };
          };
          exportarr = {
            enable = true;
            port = config.infra.clients.container-sonarr.ports.exporter;
          };
          bindMounts = {
            "/var/lib/sonarr" = {
              hostPath = "/var/lib/appdata/sonarr";
              isReadOnly = false;
            };
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
          };
          envFile = config.sops.templates."sonarr-env".path;
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
              "sonarr_exporter"
              "sonarr_rss_metrics"
              "sonarr_warn_metrics"
              "managarr_sonarr"
            ];
            extraSettings = {
              # The managarr-* sidecar in this container. Its collectors sit
              # on the loopback: the framework serves /metrics on a separate
              # address from the contract routes, so the sidecar's own port
              # answers /healthz and /changes but never this.
              sources = {
                managarr_sonarr = {
                  type = "prometheus_scrape";
                  endpoints = [ "http://127.0.0.1:9909/metrics" ];
                  scrape_interval_secs = 30;
                };
                # sonarr's Webhook connection POSTs here on loopback; the
                # sidecar republishes verbatim to MQTT so HA consumes via an
                # mqtt trigger instead of a direct webhook — no notify-time
                # dependency on HA.
                webhook_in = {
                  type = "http_server";
                  address = "127.0.0.1:9001";
                  method = "POST";
                  path = "/";
                  decoding.codec = "bytes"; # keep sonarr's JSON body verbatim → .message
                  framing.method = "bytes"; # one event per request body, not per line
                };
                sonarr_exporter = {
                  type = "prometheus_scrape";
                  endpoints = [
                    "http://127.0.0.1:${toString config.infra.clients.container-sonarr.ports.exporter}/metrics"
                  ];
                  scrape_interval_secs = 30;
                  scrape_timeout_secs = 30;
                };
              };
              sinks.webhook_mqtt = {
                type = "mqtt";
                inputs = [ "webhook_in" ];
                host = config.infra.clients.container-mosquitto.ip;
                port = config.infra.clients.container-mosquitto.ports.default;
                client_id = "vector-sonarr"; # unique per container → no broker client-id collision
                topic = "homelab/notify/sonarr";
                quality_of_service = "atleastonce";
                retain = false;
                encoding.codec = "text"; # emit .message raw (the sonarr JSON)
              };
              transforms = {
                sonarr_rss_filter = {
                  type = "filter";
                  inputs = [ "journal" ];
                  condition = ''match(to_string(.message) ?? "", r'RSS Sync Completed')'';
                };
                sonarr_rss_parse = {
                  type = "remap";
                  inputs = [ "sonarr_rss_filter" ];
                  source = ''
                    msg = to_string(.message) ?? ""
                    m = parse_regex!(msg, r'Reports found: (?P<found>[0-9]+), Reports grabbed: (?P<grabbed>[0-9]+)')
                    .rss_found = to_int!(m.found)
                    .rss_grabbed = to_int!(m.grabbed)
                  '';
                };
                sonarr_rss_metrics = {
                  type = "log_to_metric";
                  inputs = [ "sonarr_rss_parse" ];
                  metrics = [
                    {
                      type = "gauge";
                      field = "rss_found";
                      name = "sonarr_rss_reports_found";
                    }
                    {
                      type = "gauge";
                      field = "rss_grabbed";
                      name = "sonarr_rss_reports_grabbed";
                    }
                  ];
                };
                sonarr_warn_filter = {
                  type = "filter";
                  inputs = [ "journal" ];
                  condition = ''match(to_string(.message) ?? "", r'\[Warn\]|\[Error\]')'';
                };
                sonarr_warn_metrics = {
                  type = "log_to_metric";
                  inputs = [ "sonarr_warn_filter" ];
                  metrics = [
                    {
                      type = "counter";
                      field = "message";
                      name = "sonarr_warnings_total";
                    }
                  ];
                };
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/sonarr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/sonarr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@sonarr" = {
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
