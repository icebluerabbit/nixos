# modules/hosts/server/containers/youtarr.nix
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.youtarr = {
    aspect-name = "hosts.server.containers.youtarr";
    includes = [
      den.aspects.containers._.youtarr
      den.aspects.infra._.clients._.container-youtarr
      den.aspects.infra._.clients._.container-jellyfin
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
      { config, ... }:
      let
        cfg = config.containerConfigs.youtarr;
      in
      {
        systemd.services."container@youtarr" = {
          after = [ "container-wave-arr.target" ];
          before = [ "container-wave-downstream.target" ];
        };

        containerConfigs.youtarr = {

          memoryHigh = "4G";
          lldpd.enable = true;
          port = config.infra.clients.container-youtarr.ports.http;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-youtarr.mac;
          localAddress = "${config.infra.clients.container-youtarr.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          dbHost = "127.0.0.1";
          dbPasswordFile = "/run/secrets/youtarr-env";
          bindMounts = {
            "/var/lib/youtarr" = {
              hostPath = "/var/lib/appdata/youtarr";
              isReadOnly = false;
            };
            "/var/lib/mysql" = {
              hostPath = "/var/lib/appdata/youtarr-mysql";
              isReadOnly = false;
            };
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
            "/run/secrets/youtarr-env" = {
              hostPath = config.sops.templates."youtarr-env".path;
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
            lokiInputs = [
              "youtarr_route._unmatched"
              "youtarr_route.completion"
            ];
            prometheusInputs = [
              "youtarr_metrics"
              "youtarr_completion_metrics"
            ];
            extraSettings = {
              # youtarr's Apprise notifier POSTs here on loopback (json:// schema);
              # the sidecar republishes verbatim to MQTT so HA consumes via an mqtt
              # trigger instead of a direct webhook — no notify-time dependency on HA.
              sources.webhook_in = {
                type = "http_server";
                address = "127.0.0.1:9001";
                method = "POST";
                path = "/";
                decoding.codec = "bytes"; # keep Apprise's JSON body verbatim → .message
                framing.method = "bytes"; # one event per request body, not per line
              };
              sinks.webhook_mqtt = {
                type = "mqtt";
                inputs = [ "webhook_in" ];
                host = config.infra.clients.container-mosquitto.ip;
                port = config.infra.clients.container-mosquitto.ports.default;
                client_id = "vector-youtarr"; # unique per container → no broker client-id collision
                topic = "homelab/notify/youtarr";
                quality_of_service = "atleastonce";
                retain = false;
                encoding.codec = "text"; # emit .message raw (the Apprise JSON)
              };
              transforms = {
                youtarr_route = {
                  type = "route";
                  inputs = [ "journal" ];
                  route = {
                    progress = ''contains(string!(.message), "\"source\":\"yt-dlp\"") && contains(string!(.message), "\"percent\":")'';
                    completion = ''contains(string!(.message), "Marked video as completed in tracking")'';
                  };
                };
                youtarr_parse = {
                  type = "remap";
                  inputs = [ "youtarr_route.progress" ];
                  drop_on_abort = true;
                  source = ''
                    msg = replace(string!(.message), r'\x1b\[[0-9;]*[mGKFH]', "")
                    parsed, err = parse_regex(msg, r'(?P<payload>\{"percent":[^}]+\})')
                    if err != null { abort }
                    data, err = parse_json(string!(parsed.payload))
                    if err != null { abort }

                    pct = replace(replace(string!(data.percent), " ", ""), "%", "")
                    .yt_speed      = to_float!(string!(data.speed))
                    .yt_downloaded = to_float!(string!(data.downloaded))
                    .yt_total      = to_float!(string!(data.total))
                    .yt_percent    = to_float!(pct)
                  '';
                };
                youtarr_throttle = {
                  type = "throttle";
                  inputs = [ "youtarr_parse" ];
                  threshold = 1;
                  window_secs = 5;
                };
                youtarr_metrics = {
                  type = "log_to_metric";
                  inputs = [ "youtarr_throttle" ];
                  metrics = [
                    {
                      type = "gauge";
                      field = "yt_speed";
                      name = "youtarr_download_speed_bytes_per_second";
                    }
                    {
                      type = "gauge";
                      field = "yt_downloaded";
                      name = "youtarr_download_bytes_downloaded";
                    }
                    {
                      type = "gauge";
                      field = "yt_total";
                      name = "youtarr_download_total_bytes";
                    }
                    {
                      type = "gauge";
                      field = "yt_percent";
                      name = "youtarr_download_percent";
                    }
                  ];
                };
                youtarr_parse_completed = {
                  type = "remap";
                  inputs = [ "youtarr_route.completion" ];
                  drop_on_abort = true;
                  source = ''
                    msg = replace(string!(.message), r'\x1b\[[0-9;]*[mGKFH]', "")
                    parsed, err = parse_regex(msg, r'"finalVideoPath":"(?P<path>[^"]+)"')
                    if err != null { abort }

                    cat_parsed, err = parse_regex(string!(parsed.path), r'/youtube/(?P<cat>[^/]+)/')
                    if err != null { abort }
                    .yt_category = replace(string!(cat_parsed.cat), r'^__', "")
                  '';
                };
                youtarr_completion_metrics = {
                  type = "log_to_metric";
                  inputs = [ "youtarr_parse_completed" ];
                  metrics = [
                    {
                      type = "counter";
                      field = "yt_category";
                      name = "youtarr_downloads_completed_total";
                      increment_by_value = false;
                      tags = {
                        category = "{{yt_category}}";
                      };
                    }
                  ];
                };
              };
            };
          };
          youtubeOutputDir = "/mnt/media/complete/youtube";
          auth.enable = false;
          settings = {
            channelAutoDownload = true;
            channelFilesToDownload = 10;
            preferredResolution = "2160";
            videoCodec = "default";
            subtitlesEnabled = true;
            sponsorblockEnabled = true;
            downloadSocketTimeoutSeconds = 30;
            downloadThrottledRate = "100K";
            downloadRetryCount = 3;
            sleepRequests = 120;
            cookiesEnabled = true;
            customCookiesUploaded = true;
            # Apprise CLI → generic json:// schema → the Vector sidecar above.
            # youtarr has no registry entry for json://, so it falls back to the
            # plain formatter: the payload is {version,title,message,attachments,type},
            # emitted once per download run / auto-removal / test — not per video.
            notificationsEnabled = true;
            appriseUrls = [
              {
                url = "json://127.0.0.1:9001";
                name = "Homelab";
                richFormatting = false;
              }
            ];
            uuid = "c6a77a5a-ed49-4075-bb2c-ff5311261fb9";
            # Jellyfin integration. The API key is a secret — kept out of the
            # store via secretSettings below (injected at runtime from youtarr-env).
            jellyfinEnabled = true;
            jellyfinUrl = "http://${config.infra.clients.container-jellyfin.ip}:${toString config.infra.clients.container-jellyfin.ports.http}";
            jellyfinUserId = "036a96a4fab44f31af0e3cc6ec0dad95";
            # YouTube library GUID (Jellyfin /Library/VirtualFolders ItemId),
            # created by the jellyfin configuratarr sync. Persistent across restarts.
            jellyfinVideoLibraryIds = [ "34f331a89ce405e2b877d68d5ee4d4a2" ];
          };
          # jellyfinApiKey injected at runtime from JELLYFIN_API_KEY (youtarr-env),
          # never rendered into the world-readable store config.json.
          secretSettings.jellyfinApiKey = "JELLYFIN_API_KEY";
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/youtarr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/youtarr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "d /var/lib/appdata/youtarr-mysql 0700 ${toString cfg.mysqlUid} ${toString cfg.mysqlGid} - -"
          "Z /var/lib/appdata/youtarr-mysql 0700 ${toString cfg.mysqlUid} ${toString cfg.mysqlGid} - -"
        ];
      };
  };
}
