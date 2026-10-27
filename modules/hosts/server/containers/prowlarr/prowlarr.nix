# prowlarr — host configuration override for prowlarr container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.prowlarr = {
    aspect-name = "hosts.server.containers.prowlarr";
    includes = [
      den.aspects.containers._.prowlarr
      den.aspects.infra._.clients._.container-prowlarr
      den.aspects.infra._.clients._.container-radarr
      den.aspects.infra._.clients._.container-sonarr
      den.aspects.infra._.clients._.container-lidarr
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.infra._.clients._.container-flaresolverr
      den.aspects.infra._.clients._.container-home-assistant
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
      { config, ... }:
      let
        cfg = config.containerConfigs.prowlarr;
      in
      {
        containerConfigs.prowlarr = {
          # Where managarr-prowlarr relays what it sees. The read half works
          # without this; relaying is what makes an event reach managarr in
          # seconds rather than at the next hourly pass.
          managarrUrl = "http://${config.infra.clients.container-managarr.ip}:${toString config.infra.clients.container-managarr.ports.default}";

          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-prowlarr.mac;
          localAddress = "${config.infra.clients.container-prowlarr.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-prowlarr.ports.default;
          dbHost = config.infra.clients.container-postgres.ip;
          dbPort = config.infra.clients.container-postgres.ports.default;
          configuratarr = {
            enable = true;
            waitForHealthy = true;
            settings = {
              prowlarr = {
                type = "prowlarr-v1";
                url = "http://localhost:${toString config.infra.clients.container-prowlarr.ports.default}";
                api_key = "\${env.PROWLARR__AUTH__APIKEY}";
                tags = [
                  { label = "cloudflare"; }
                  { label = "movies"; }
                  { label = "tv"; }
                  { label = "music"; }
                ];
                notifications = [
                  (import ./notifications/_home-assistant.nix { inherit config; })
                  (import ./notifications/_managarr.nix { inherit config; })
                ];
                indexer_proxies = [
                  (import ./indexer-proxies/_flaresolverr.nix { inherit config; })
                ];
                indexers = [
                  (import ./indexers/_pretome.nix { inherit config; })
                  (import ./indexers/_torrentleech.nix { inherit config; })
                  (import ./indexers/_retrotoon.nix { inherit config; })
                  (import ./indexers/_darkpeers.nix { inherit config; })
                  (import ./indexers/_yuscene.nix { inherit config; })
                  (import ./indexers/_seedpool.nix { inherit config; })
                ];
                download_clients = [
                  (import ./download-clients/_qbittorrent.nix { inherit config; })
                ];
                applications = [
                  (import ./applications/_radarr.nix { inherit config; })
                  (import ./applications/_sonarr.nix { inherit config; })
                  (import ./applications/_lidarr.nix { inherit config; })
                ];
              };
            };
          };
          exportarr = {
            enable = true;
            port = config.infra.clients.container-prowlarr.ports.exporter;
          };
          bindMounts = {
            "/var/lib/prowlarr" = {
              hostPath = "/var/lib/appdata/prowlarr";
              isReadOnly = false;
            };
          };
          envFile = config.sops.templates."prowlarr-env".path;
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
              "prowlarr_exporter"
              "prowlarr_warn_metrics"
              "managarr_prowlarr"
            ];
            extraSettings = {
              # The managarr-* sidecar in this container. Its collectors sit
              # on the loopback: the framework serves /metrics on a separate
              # address from the contract routes, so the sidecar's own port
              # answers /healthz and /changes but never this.
              sources = {
                managarr_prowlarr = {
                  type = "prometheus_scrape";
                  endpoints = [ "http://127.0.0.1:9909/metrics" ];
                  scrape_interval_secs = 30;
                };
                # prowlarr's Webhook connection POSTs here on loopback; the
                # sidecar republishes verbatim to MQTT so HA consumes via an
                # mqtt trigger instead of a direct webhook — no notify-time
                # dependency on HA.
                webhook_in = {
                  type = "http_server";
                  address = "127.0.0.1:9001";
                  method = "POST";
                  path = "/";
                  decoding.codec = "bytes"; # keep prowlarr's JSON body verbatim → .message
                  framing.method = "bytes"; # one event per request body, not per line
                };
                prowlarr_exporter = {
                  type = "prometheus_scrape";
                  endpoints = [
                    "http://127.0.0.1:${toString config.infra.clients.container-prowlarr.ports.exporter}/metrics"
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
                client_id = "vector-prowlarr"; # unique per container → no broker client-id collision
                topic = "homelab/notify/prowlarr";
                quality_of_service = "atleastonce";
                retain = false;
                encoding.codec = "text"; # emit .message raw (the prowlarr JSON)
              };
              transforms = {
                prowlarr_warn_filter = {
                  type = "filter";
                  inputs = [ "journal" ];
                  condition = ''match(to_string(.message) ?? "", r'\[Warn\]|\[Error\]')'';
                };
                prowlarr_warn_metrics = {
                  type = "log_to_metric";
                  inputs = [ "prowlarr_warn_filter" ];
                  metrics = [
                    {
                      type = "counter";
                      field = "message";
                      name = "prowlarr_warnings_total";
                    }
                  ];
                };
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/prowlarr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/prowlarr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@prowlarr" = {
          before = [ "container-wave-apps.target" ];
          wants = [ "container@postgresql.service" ];
          after = [
            "container@postgresql.service"
            "container-wave-core.target"
          ];
        };
      };
  };
}
