# modules/hosts/server/containers/home-assistant/home-assistant.nix
{ den, inputs, ... }:
{
  den.aspects.hosts._.server._.containers._.home-assistant = {
    aspect-name = "hosts.server.containers.home-assistant";
    includes = [
      den.aspects.containers._.home-assistant
      den.aspects.infra._.clients._.container-home-assistant
      den.aspects.infra._.clients._.container-caddy
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.iot
      den.aspects.infra._.networks._.trust
      den.aspects.containers._.postgresql
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.iot
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
      den.aspects.infra._.clients._.container-zwave
      den.aspects.infra._.clients._.container-mosquitto
      den.aspects.infra._.clients._.container-tuwunel
    ];

    nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        cfg = config.containerConfigs.home-assistant;
      in
      {
        containerConfigs.home-assistant = {

          lldpd.enable = true;
          hostBridge = "br-iot";
          macAddress = config.infra.clients.container-home-assistant.mac;
          localAddress = "${config.infra.clients.container-home-assistant.ip}/${toString config.infra.vlans.iot.subnetPrefix}";
          gateway = config.infra.vlans.iot.staticAddresses.gateway;
          port = config.infra.clients.container-home-assistant.ports.http;
          extraPackages = ps: [
            ps.ha-silabs-firmware-client
            ps.universal-silabs-flasher
          ];
          areas = [
            {
              area_id = "bedroom";
              name = "Bedroom";
              icon = "mdi:bed";
            }
            {
              area_id = "server_rack";
              name = "Server Rack";
              icon = "mdi:server";
            }
            {
              area_id = "living";
              name = "Living Room";
              icon = "mdi:sofa";
            }
          ];
          dbHost = config.infra.clients.container-postgres.ip;
          dbPort = config.infra.clients.container-postgres.ports.default;
          seedEnvironmentFile = "/run/secrets/seed-env";
          persons = [
            {
              id = "admin";
              device_trackers = [ "device_tracker.iphone" ];
            }
          ];
          bindMounts = {
            "/var/lib/hass" = {
              hostPath = "/var/lib/appdata/home-assistant";
              isReadOnly = false;
            };
            "/run/ca/ca.crt" = {
              hostPath = toString (inputs.self + "/certs/ca.crt");
              isReadOnly = true;
            };
            "/run/secrets/hass-vector-env" = {
              hostPath = config.sops.templates."home-assistant-vector-env".path;
              isReadOnly = true;
            };
            "/var/lib/hass/secrets.yaml" = {
              hostPath = config.sops.templates."home-assistant-secrets".path;
              isReadOnly = true;
            };
            "/run/secrets/seed-env" = {
              hostPath = config.sops.templates."home-assistant-seed-env".path;
              isReadOnly = true;
            };
          };
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          trustedProxies = [
            config.infra.clients.container-caddy.ip
          ];
          extraComponents = [
            "zwave_js"
            "mqtt"
            "hue"
            "met"
            "radio_browser"
            "google_translate"
            "esphome"
            "mobile_app"
            "homekit"
            "homekit_controller"
            # zeroconf discovers a Cast device at every start; without this the
            # flow can't load (no pychromecast) and throws UnknownHandler.
            "cast"
            "unifi"
            "playstation_network"
            "matrix"
            # Serves /api/webhook/<id> for the *arr notification automation.
            "webhook"
            "imap"
            "github"
          ];
          configEntries = [
            (import ./config-entries/_mqtt.nix { inherit config; })
            (import ./config-entries/_zwave-js.nix { inherit config; })
            (import ./config-entries/_unifi.nix { inherit config; })
            (import ./config-entries/_gmail-personal.nix { })
            (import ./config-entries/_gmail-dev.nix { })
            (import ./config-entries/_github.nix { inherit lib; })
          ];
          customComponents = [ pkgs.home-assistant-custom-components.auth_oidc ];
          extraConfig = {
            # Force-set up the webhook integration (dependency-only, empty config
            # schema) so /api/webhook/<id> is served for the *arr notifications.
            webhook = { };
            matrix = {
              homeserver = "http://${config.infra.clients.container-tuwunel.ip}:${toString config.infra.clients.container-tuwunel.ports.default}";
              username = "@ha-bot:matrix.services.internal";
              password = "!secret matrix_ha_bot_password";
              # ha-bot must be joined to every room it posts to.
              rooms = [
                "#radarr:matrix.services.internal"
                "#sonarr:matrix.services.internal"
                "#lidarr:matrix.services.internal"
                "#jellyfin:matrix.services.internal"
                "#youtarr:matrix.services.internal"
                "#prowlarr:matrix.services.internal"
                "#bazarr:matrix.services.internal"
                "#autobrr:matrix.services.internal"
                "#myanonamouse:matrix.services.internal"
                "#qbittorrent:matrix.services.internal"
                "#cross-seed:matrix.services.internal"
                "#cleanuparr:matrix.services.internal"
                "#seasonpackarr:matrix.services.internal"
                "#soulseek:matrix.services.internal"
                "#bindery:matrix.services.internal"
                "#komga:matrix.services.internal"
                "#audiobookshelf:matrix.services.internal"
                "#automations:matrix.services.internal"
                "#home-assistant:matrix.services.internal"
                "#alerts:matrix.services.internal"
                "#vpn-gw:matrix.services.internal"
                "#unifi:matrix.services.internal"
                "#dns:matrix.services.internal"
                "#clients:matrix.services.internal"
                "#nixos:matrix.services.internal"
                "#backups:matrix.services.internal"
                "#inbox:matrix.services.internal"
                "#releases:matrix.services.internal"
                "#pull-requests:matrix.services.internal"
                "#issues:matrix.services.internal"
                "#commits:matrix.services.internal"
                "#stars:matrix.services.internal"
              ];
              commands = [
                (import ./matrix-commands/_autobrr-grab.nix)
                (import ./matrix-commands/_mail-unsub.nix)
              ];
            };
            rest_command =
              (import ./rest-commands/_matrix-login.nix { inherit config; })
              // (import ./rest-commands/_matrix-get-event.nix { inherit config; })
              // (import ./rest-commands/_qbt-add.nix { inherit config; })
              // (import ./rest-commands/_vpn-rotate.nix { inherit config; })
              // (import ./rest-commands/_prometheus-query.nix { inherit config; })
              // (import ./rest-commands/_mail-unsubscribe.nix { });
            automation = [
              (import ./automations/_moes-button-1-living-33.nix { })
              (import ./automations/_moes-button-2-living-66.nix { })
              (import ./automations/_moes-button-3-living-100.nix { })
              (import ./automations/_radarr-webhook.nix { })
              (import ./automations/_sonarr-webhook.nix { })
              (import ./automations/_lidarr-webhook.nix { })
              (import ./automations/_soulseek-webhook.nix { })
              (import ./automations/_jellyfin-webhook.nix { })
              (import ./automations/_youtarr-webhook.nix { })
              (import ./automations/_prowlarr-webhook.nix { })
              (import ./automations/_autobrr-webhook.nix { })
              (import ./automations/_mousetrap-webhook.nix { })
              (import ./automations/_bindery-webhook.nix { })
              (import ./automations/_qbittorrent_webhook.nix { })
              (import ./automations/_bazarr-webhook.nix { })
              (import ./automations/_automations-relay.nix { })
              (import ./automations/_ha-startup.nix { })
              (import ./automations/_ha-updates.nix { })
              (import ./automations/_nixos-webhook.nix { })
              (import ./automations/_backup-webhook.nix { })
              (import ./automations/_grafana-alerts-webhook.nix { inherit lib; })
              (import ./automations/_dns-events-webhook.nix { })
              (import ./automations/_dns-digest.nix { })
              (import ./automations/_cross-seed-webhook.nix { })
              (import ./automations/_cleanuparr-webhook.nix { })
              (import ./automations/_seasonpackarr-webhook.nix { })
              (import ./automations/_presence.nix { })
              (import ./automations/_new-client.nix { })
              (import ./automations/_device-down.nix { })
              (import ./automations/_away-motion-alert.nix { })
              (import ./automations/_living-room-auto-light.nix { })
              (import ./automations/_mail-relay.nix { })
              (import ./automations/_mail-unsubscribe.nix { })
              (import ./automations/_github-releases.nix { inherit lib; })
              (import ./automations/_github-pull-requests.nix { inherit lib; })
              (import ./automations/_github-issues.nix { inherit lib; })
              (import ./automations/_github-commits.nix { inherit lib; })
              (import ./automations/_github-stars.nix { inherit lib; })
            ];
            prometheus = {
              filter = {
                include_domains = [
                  "binary_sensor"
                  "climate"
                  "cover"
                  "device_tracker"
                  "fan"
                  "humidifier"
                  "input_boolean"
                  "light"
                  "lock"
                  "media_player"
                  "person"
                  "sensor"
                  "switch"
                  "update"
                  "vacuum"
                ];
              };
            };
            auth_oidc = {
              client_id = "homeassistant";
              discovery_url = "https://authelia.caddy.${config.infra.vlans.apps.domain}/.well-known/openid-configuration";
              features.default_redirect = true;
              network.tls_ca_path = "/run/ca/ca.crt";
            };
            homeassistant = {
              name = "Home";
              latitude = "!secret latitude";
              longitude = "!secret longitude";
              elevation = 0;
              unit_system = "us_customary";
              time_zone = "America/New_York";
              currency = "USD";
              country = "US";
              language = "en";
              radius = 100;
              auth_providers = [
                { type = "homeassistant"; }
                {
                  type = "trusted_networks";
                  trusted_networks = [
                    "${config.infra.vlans.trust.subnet}/${toString config.infra.vlans.trust.subnetPrefix}"
                  ];
                  allow_bypass_login = true;
                }
              ];
            };
          };
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            prometheusInputs = [ "hass_metrics" ];
            environmentFile = "/run/secrets/hass-vector-env";
            validateConfig = false;
            extraSettings = {
              sources.hass_metrics = {
                type = "prometheus_scrape";
                endpoints = [
                  "http://localhost:${toString config.containerConfigs.home-assistant.port}/api/prometheus"
                ];
                scrape_interval_secs = 60;
                auth = {
                  strategy = "bearer";
                  token = "$HASS_TOKEN";
                };
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/home-assistant 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/home-assistant 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@home-assistant" = {
          before = [ "container-wave-core.target" ];
          wants = [ "container@postgresql.service" ];
          after = [
            "container@postgresql.service"
            "container-wave-foundation.target"
          ];
        };
      };
  };
}
