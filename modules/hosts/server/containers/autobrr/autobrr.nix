# autobrr — host configuration override for autobrr container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.autobrr = {
    aspect-name = "hosts.server.containers.autobrr";
    includes = [
      den.aspects.containers._.autobrr
      den.aspects.infra._.clients._.container-autobrr
      den.aspects.infra._.clients._.container-cross-seed
      den.aspects.infra._.clients._.container-mousetrap
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.infra._.clients._.container-seasonpackarr
      den.aspects.infra._.clients._.container-sonarr
      den.aspects.infra._.clients._.container-radarr
      den.aspects.infra._.clients._.container-lidarr
      den.aspects.infra._.clients._.container-home-assistant
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
      { config, lib, ... }:
      let
        cfg = config.containerConfigs.autobrr;
        port = config.infra.clients.container-autobrr.ports.default;
      in
      {
        containerConfigs.autobrr = {
          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-autobrr.mac;
          localAddress = "${config.infra.clients.container-autobrr.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          inherit port;
          secretFile = "/run/secrets/autobrr-session-secret";
          logLevel = "DEBUG";
          envFile = config.sops.templates."autobrr-env".path;
          # Extra indexer definitions for trackers autobrr does not ship one for.
          customDefinitions = ./definitions;
          configuratarr = {
            enable = true;
            waitForHealthy = true;
            settings.autobrr = {
              type = "autobrr-v1";
              url = "http://localhost:${toString port}";
              api_key = "\${env.AUTOBRR_API_TOKEN}";
              download_clients = [
                (import ./download-clients/_sonarr.nix { inherit config; })
                (import ./download-clients/_radarr.nix { inherit config; })
                (import ./download-clients/_lidarr.nix { inherit config; })
                (import ./download-clients/_qbittorrent.nix { inherit config; })
              ];
              indexers = [
                (import ./indexers/_pretome.nix { inherit config; })
                (import ./indexers/_torrentleech.nix { inherit config; })
                (import ./indexers/_myanonamouse.nix { inherit config; })
                (import ./indexers/_digitalcore.nix { inherit config; })
                (import ./indexers/_retrotoon.nix { inherit config; })
                (import ./indexers/_darkpeers.nix { inherit config; })
              ];
              irc_networks = [
                (import ./irc-networks/_pretome.nix { inherit config; })
                (import ./irc-networks/_torrentleech.nix { inherit config; })
                (import ./irc-networks/_myanonamouse.nix { inherit config; })
                (import ./irc-networks/_digitalcore.nix { inherit config; })
                (import ./irc-networks/_retrotoon.nix { inherit config; })
                (import ./irc-networks/_darkpeers.nix { inherit config; })
              ];
              filters = [
                # Priority 100 — must be evaluated before every TV filter below.
                (import ./filters/_seasonpackarr.nix { inherit config; })
                (import ./filters/_torrentleech-tv.nix { inherit config; })
                (import ./filters/_torrentleech-movies.nix { inherit config; })
                (import ./filters/_torrentleech-music.nix { inherit config; })
                (import ./filters/_mam.nix { inherit config; })
                (import ./filters/_digitalcore-tv.nix { inherit config; })
                (import ./filters/_digitalcore-movies.nix { inherit config; })
                (import ./filters/_retrotoon.nix { inherit config; })
                (import ./filters/_pretome-tv.nix { inherit config; })
                (import ./filters/_pretome-movies.nix { inherit config; })
                (import ./filters/_pretome-music.nix { inherit config; })
                (import ./filters/_darkpeers-tv.nix { inherit config; })
                (import ./filters/_darkpeers-movies.nix { inherit config; })
                (import ./filters/_darkpeers-music.nix { inherit config; })
              ];
              notifications = [
                (import ./notifications/_home-assistant.nix { inherit config; })
              ];
            };
          };
          bindMounts = {
            "/var/lib/autobrr" = {
              hostPath = "/var/lib/appdata/autobrr";
              isReadOnly = false;
            };
            "/run/secrets/autobrr-session-secret" = {
              hostPath = config.sops.secrets.autobrr-session-secret.path;
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
            # Vector polls the autobrr API for metrics (no packaged exporter);
            # X-API-Token comes from AUTOBRR_API_TOKEN in the mounted autobrr-env.
            environmentFile = "/run/secrets/autobrr-env";
            prometheusInputs = [
              "autobrr_release_stats_metrics"
              "autobrr_irc_metrics"
            ];
            extraSettings =
              lib.recursiveUpdate
                (import ./_exporter.nix {
                  inherit port;
                })
                {
                  # autobrr's WEBHOOK notification POSTs here on loopback; the sidecar
                  # republishes verbatim to MQTT so HA consumes via an mqtt trigger
                  # instead of a direct webhook — no notify-time dependency on HA.
                  sources.webhook_in = {
                    type = "http_server";
                    address = "127.0.0.1:9001";
                    method = "POST";
                    path = "/";
                    decoding.codec = "bytes"; # keep autobrr's JSON body verbatim → .message
                    framing.method = "bytes"; # one event per request body, not per line
                  };
                  sinks.webhook_mqtt = {
                    type = "mqtt";
                    inputs = [ "webhook_in" ];
                    host = config.infra.clients.container-mosquitto.ip;
                    port = config.infra.clients.container-mosquitto.ports.default;
                    client_id = "vector-autobrr"; # unique per container → no broker client-id collision
                    topic = "homelab/notify/autobrr";
                    quality_of_service = "atleastonce";
                    retain = false;
                    encoding.codec = "text"; # emit .message raw (the autobrr JSON)
                  };
                };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/autobrr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/autobrr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@autobrr" = {
          before = [ "container-wave-downstream.target" ];
          wants = [ "container@qbittorrent.service" ];
          after = [
            "container@qbittorrent.service"
            "container-wave-arr.target"
          ];
        };
      };
  };
}
