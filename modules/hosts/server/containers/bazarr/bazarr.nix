# bazarr — host configuration override for bazarr container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.bazarr = {
    aspect-name = "hosts.server.containers.bazarr";
    includes = [
      den.aspects.containers._.bazarr
      den.aspects.infra._.clients._.container-bazarr
      den.aspects.infra._.clients._.container-sonarr
      den.aspects.infra._.clients._.container-radarr
      den.aspects.infra._.clients._.container-postgres
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
        cfg = config.containerConfigs.bazarr;
      in
      {
        containerConfigs.bazarr = {
          # Where managarr-bazarr relays what it sees. The read half works
          # without this; relaying is what makes an event reach managarr in
          # seconds rather than at the next hourly pass.
          managarrUrl = "http://${config.infra.clients.container-managarr.ip}:${toString config.infra.clients.container-managarr.ports.default}";

          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-bazarr.mac;
          localAddress = "${config.infra.clients.container-bazarr.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-bazarr.ports.default;
          dbHost = config.infra.clients.container-postgres.ip;
          dbPort = config.infra.clients.container-postgres.ports.default;
          configuratarr = {
            enable = true;
            waitForHealthy = true;
            settings.bazarr = {
              type = "bazarr-v1";
              url = "http://localhost:${toString config.infra.clients.container-bazarr.ports.default}";
              api_key = "\${env.BAZARR_API_KEY}";
              notifications = {
                providers = [
                  (import ./notifications/_home-assistant.nix { inherit config; })
                ];
              };
              settings = {
                general = {
                  use_sonarr = true;
                  use_radarr = true;
                  serie_default_enabled = true;
                  serie_default_profile = "1";
                  movie_default_enabled = true;
                  movie_default_profile = "1";
                  enabled_providers = [
                    "podnapisi"
                    "tvsubtitles"
                    "opensubtitlescom"
                    # gestdown = addic7ed catalog via API (no account, no captcha).
                    "gestdown"
                    "subdl"
                    # addic7ed login requires an anti-captcha provider or browser
                    # cookies (username/password alone won't authenticate). Disabled
                    # until one is wired.
                    # "addic7ed"
                  ];
                };
                # podnapisi, tvsubtitles + gestdown need no credentials — listing the id is enough.
                opensubtitlescom = {
                  username = "\${env.OPENSUBTITLES_USERNAME}";
                  password = "\${env.OPENSUBTITLES_PASSWORD}";
                  use_hash = true;
                };
                subdl = {
                  api_key = "\${env.SUBDL_API_KEY}";
                };
                # addic7ed = {
                #   username = "\${env.ADDIC7ED_USERNAME}";
                #   password = "\${env.ADDIC7ED_PASSWORD}";
                # };
                sonarr = {
                  ip = config.infra.clients.container-sonarr.ip;
                  port = config.infra.clients.container-sonarr.ports.default;
                  apikey = "\${env.SONARR_APIKEY}";
                };
                radarr = {
                  ip = config.infra.clients.container-radarr.ip;
                  port = config.infra.clients.container-radarr.ports.default;
                  apikey = "\${env.RADARR_APIKEY}";
                };
              };
              languages = {
                enabled_languages = [ "en" ];
                language_profiles = [
                  {
                    profile_id = 1;
                    name = "English";
                    cutoff = 65535;
                    items = [
                      {
                        id = 1;
                        language = "en";
                        forced = "False";
                        hi = "False";
                      }
                    ];
                  }
                ];
              };
            };
          };
          bindMounts = {
            "/var/lib/bazarr" = {
              hostPath = "/var/lib/appdata/bazarr";
              isReadOnly = false;
            };
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
          };
          envFile = config.sops.templates."bazarr-env".path;
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            # Vector polls the Bazarr API for metrics (no packaged exporter);
            # X-API-KEY comes from BAZARR_API_KEY in the mounted bazarr-env.
            environmentFile = "/run/secrets/bazarr-env";
            prometheusInputs = [
              "bazarr_badges_metrics"
              "bazarr_history_metrics"
              "managarr_bazarr"
            ];
            extraSettings =
              lib.recursiveUpdate
                (import ./_exporter.nix {
                  port = config.infra.clients.container-bazarr.ports.default;
                })
                {
                  # The managarr-bazarr sidecar in this container. Its
                  # collectors sit on the loopback: the framework serves
                  # /metrics on a separate address from the contract routes, so
                  # the sidecar's own port answers /healthz and /changes but
                  # never this.
                  sources.managarr_bazarr = {
                    type = "prometheus_scrape";
                    endpoints = [ "http://127.0.0.1:9909/metrics" ];
                    scrape_interval_secs = 30;
                  };

                  # bazarr's Apprise "JSON" provider POSTs here on loopback; the sidecar
                  # republishes verbatim to MQTT so HA consumes via an mqtt trigger
                  # instead of a direct webhook — no notify-time dependency on HA.
                  sources.webhook_in = {
                    type = "http_server";
                    address = "127.0.0.1:9001";
                    method = "POST";
                    path = "/";
                    decoding.codec = "bytes"; # keep the Apprise JSON body verbatim → .message
                    framing.method = "bytes"; # one event per request body, not per line
                  };
                  sinks.webhook_mqtt = {
                    type = "mqtt";
                    inputs = [ "webhook_in" ];
                    host = config.infra.clients.container-mosquitto.ip;
                    port = config.infra.clients.container-mosquitto.ports.default;
                    client_id = "vector-bazarr"; # unique per container → no broker client-id collision
                    topic = "homelab/notify/bazarr";
                    quality_of_service = "atleastonce";
                    retain = false;
                    encoding.codec = "text"; # emit .message raw (the Apprise JSON)
                  };
                };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/bazarr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/bazarr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@bazarr" = {
          before = [ "container-wave-downstream.target" ];
          wants = [ "container@postgresql.service" ];
          after = [
            "container@postgresql.service"
            "remote-fs.target"
            "container-wave-arr.target"
          ];
        };
      };
  };
}
