# caddy — host configuration override for caddy reverse proxy container
{ den, inputs, ... }:
{
  den.aspects.hosts._.server._.containers._.caddy = {
    aspect-name = "hosts.server.containers.caddy";
    includes = [
      den.aspects.containers._.caddy
      den.aspects.infra._.clients._.container-caddy
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.apps
      den.aspects.hosts._.server._.networking._.vlans._.apps
      den.aspects.infra._.clients._.container-loki

      # VirtualHost targets
      den.aspects.containers._.qui
      den.aspects.infra._.clients._.container-qui
      den.aspects.containers._.qbittorrent
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.containers._.slskd
      den.aspects.infra._.clients._.container-slskd
      den.aspects.containers._.soularr
      den.aspects.infra._.clients._.container-soularr
      den.aspects.containers._.pgadmin
      den.aspects.infra._.clients._.container-pgadmin
      den.aspects.containers._.radarr
      den.aspects.infra._.clients._.container-radarr
      den.aspects.containers._.calibre-web
      den.aspects.infra._.clients._.container-calibre-web
      den.aspects.containers._.jellyfin
      den.aspects.infra._.clients._.container-jellyfin
      den.aspects.containers._.kiwix
      den.aspects.infra._.clients._.container-kiwix
      den.aspects.containers._.gitea
      den.aspects.infra._.clients._.container-gitea
      den.aspects.containers._.home-assistant
      den.aspects.infra._.clients._.container-home-assistant
      den.aspects.containers._.zwave
      den.aspects.infra._.clients._.container-zwave
      den.aspects.containers._.zigbee2mqtt
      den.aspects.infra._.clients._.container-zigbee2mqtt
      den.aspects.containers._.pi-hole
      den.aspects.infra._.clients._.container-pi-hole
      den.aspects.containers._.lidarr
      den.aspects.infra._.clients._.container-lidarr
      den.aspects.containers._.beets
      den.aspects.infra._.clients._.container-beets
      den.aspects.containers._.readarr
      den.aspects.infra._.clients._.container-readarr
      den.aspects.containers._.sonarr
      den.aspects.infra._.clients._.container-sonarr
      den.aspects.containers._.bazarr
      den.aspects.infra._.clients._.container-bazarr
      den.aspects.containers._.prowlarr
      den.aspects.infra._.clients._.container-prowlarr
      den.aspects.containers._.stash
      den.aspects.infra._.clients._.container-stash
      den.aspects.containers._.homepage
      den.aspects.infra._.clients._.container-homepage
      den.aspects.containers._.youtarr
      den.aspects.infra._.clients._.container-youtarr
      den.aspects.containers._.grafana
      den.aspects.infra._.clients._.container-grafana
      den.aspects.containers._.prometheus
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.containers._.tempo
      den.aspects.infra._.clients._.container-tempo
      den.aspects.containers._.opencode
      den.aspects.infra._.clients._.container-opencode
      den.aspects.containers._.openchamber
      den.aspects.infra._.clients._.container-openchamber
      den.aspects.containers._.code-server
      den.aspects.infra._.clients._.container-code-server
      den.aspects.containers._.authelia
      den.aspects.infra._.clients._.container-authelia
      den.aspects.containers._.lldap
      den.aspects.infra._.clients._.container-lldap
      den.aspects.containers._.tuwunel
      den.aspects.infra._.clients._.container-tuwunel
      den.aspects.containers._.hermes
      den.aspects.infra._.clients._.container-hermes
      den.aspects.containers._.cinny
      den.aspects.infra._.clients._.container-cinny
      den.aspects.containers._.thelounge
      den.aspects.infra._.clients._.container-thelounge
      den.aspects.containers._.cross-seed
      den.aspects.infra._.clients._.container-cross-seed
      den.aspects.containers._.cleanuparr
      den.aspects.infra._.clients._.container-cleanuparr
      den.aspects.containers._.autobrr
      den.aspects.infra._.clients._.container-autobrr
    ];

    nixos =
      {
        config,
        lib,
        ...
      }:
      let
        vhostOverrides = {
          authelia = {
            requireAuth = false;
          };
          homepage = {
            requireAuth = false;
          };
          tuwunel = {
            requireAuth = false;
          };
          qui = {
            stream = true;
          };
          code-server = {
            stream = true; # long-lived IDE connections
            # code-server serves codicon.ttf as text/plain, which makes the browser
            # treat the terminal's glyph font as opaque and taints the WebGL canvas.
            extraDirectives = [
              "@fonts path *.ttf"
              "header @fonts Content-Type font/ttf"
            ];
          };
          gitea = {
            headerMap = {
              "X-Webauth-User" = "Remote-User";
              "X-Webauth-Email" = "Remote-Email";
            };
          };
        };
      in
      {
        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/caddy 0755 root root - -"
        ];

        systemd.services."container@caddy" = {
          after = [ "container-wave-foundation.target" ];
          before = [ "container-wave-core.target" ];
        };

        containerConfigs.caddy = {

          lldpd.enable = true;
          enableTelemetry = true;
          hostBridge = "br-apps";
          macAddress = config.infra.clients.container-caddy.mac;
          localAddress = "${config.infra.clients.container-caddy.ip}/${toString config.infra.vlans.apps.subnetPrefix}";
          gateway = config.infra.vlans.apps.staticAddresses.gateway;
          httpPort = config.infra.clients.container-caddy.ports.http;
          httpsPort = config.infra.clients.container-caddy.ports.https;
          adminPort = config.infra.clients.container-caddy.ports.admin;
          domainSuffix = "caddy.${config.infra.vlans.apps.domain}";
          authelia = {
            enable = true;
            address = "http://${config.infra.clients.container-authelia.ip}:${toString config.infra.clients.container-authelia.ports.default}";
          };
          virtualHosts = lib.mapAttrs' (name: client: {
            name = lib.removePrefix "container-" name;
            value = {
              inherit (client) ip;
              port = client.ports.${client.proxy.port};
            }
            // (vhostOverrides.${lib.removePrefix "container-" name} or { });
          }) (lib.filterAttrs (_: c: c.proxy != null) config.infra.clients);
          tlsCertFile = "/run/caddy-tls/caddy.crt";
          tlsKeyFile = "/run/caddy-tls/caddy.key";
          bindMounts = {
            "/var/lib/caddy" = {
              hostPath = "/var/lib/appdata/caddy";
              isReadOnly = false;
            };
            "/run/caddy-tls/caddy.crt" = {
              hostPath = toString (inputs.self + "/certs/caddy.crt");
              isReadOnly = true;
            };
            "/run/caddy-tls/caddy.key" = {
              hostPath = config.sops.secrets."caddy-tls-key".path;
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
            prometheusInputs = [ "caddy_metrics" ];
            extraSettings = {
              sources.caddy_metrics = {
                type = "prometheus_scrape";
                endpoints = [
                  "http://localhost:${toString config.infra.clients.container-caddy.ports.admin}/metrics"
                ];
                scrape_interval_secs = 60;
              };
            };
          };
        };
      };
  };
}
