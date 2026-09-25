# homepage — host configuration override for the homepage container.
#
# This file is the composer: it wires the container runtime (network, secrets,
# logging) and assembles the dashboard from the `_`-prefixed partials in this
# directory (import-tree ignores `_` files, so they are pulled in manually
# below). Each partial receives `ctx` and returns a piece of the config:
#   _settings / _info / _bookmarks  → the chrome
#   _overview / _media / _downloads / _automation / _infra / _observability /
#   _network                         → one service group each (assigned to a tab).
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.homepage = {
    aspect-name = "hosts.server.containers.homepage";
    includes = [
      den.aspects.containers._.homepage
      den.aspects.infra._.clients._.container-homepage
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.apps

      # All VLANs — for the Network overview + cross-VLAN domains.
      den.aspects.infra._.networks._.mgmt
      den.aspects.infra._.networks._.trust
      den.aspects.infra._.networks._.iot
      den.aspects.infra._.networks._.guest
      den.aspects.infra._.networks._.storage
      den.aspects.infra._.networks._.work
      den.aspects.infra._.networks._.game
      den.aspects.infra._.networks._.dmz
      den.aspects.infra._.networks._.apps
      den.aspects.infra._.networks._.media
      den.aspects.infra._.networks._.download
      den.aspects.infra._.networks._.services

      # UniFi devices + NAS + server, for the Network group.
      den.aspects.infra._.devices._.dream-machine-pro-max
      den.aspects.infra._.devices._.u7-pro-xg
      den.aspects.infra._.devices._.usw-pro-hd-24-poe
      den.aspects.infra._.devices._.usw-flex-2-5g-8
      den.aspects.infra._.clients._.unas-pro
      den.aspects.infra._.clients._.server

      # Every container client we read an ip/port from (widget / customapi /
      # prometheus / health targets). esphome intentionally omitted.
      den.aspects.infra._.clients._.container-authelia
      den.aspects.infra._.clients._.container-autobrr
      den.aspects.infra._.clients._.container-bazarr
      den.aspects.infra._.clients._.container-beets
      den.aspects.infra._.clients._.container-caddy
      den.aspects.infra._.clients._.container-calibre-web
      den.aspects.infra._.clients._.container-cinny
      den.aspects.infra._.clients._.container-code-server
      den.aspects.infra._.clients._.container-cross-seed
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.clients._.container-flaresolverr
      den.aspects.infra._.clients._.container-gitea
      den.aspects.infra._.clients._.container-grafana
      den.aspects.infra._.clients._.container-hermes
      den.aspects.infra._.clients._.container-home-assistant
      den.aspects.infra._.clients._.container-jellyfin
      den.aspects.infra._.clients._.container-kiwix
      den.aspects.infra._.clients._.container-bindery
      den.aspects.infra._.clients._.container-komga
      den.aspects.infra._.clients._.container-audiobookshelf
      den.aspects.infra._.clients._.container-lidarr
      den.aspects.infra._.clients._.container-lldap
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-mosquitto
      den.aspects.infra._.clients._.container-openchamber
      den.aspects.infra._.clients._.container-opencode
      den.aspects.infra._.clients._.container-pgadmin
      den.aspects.infra._.clients._.container-pi-hole
      den.aspects.infra._.clients._.container-postgres
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-prowlarr
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.infra._.clients._.container-qui
      den.aspects.infra._.clients._.container-radarr
      den.aspects.infra._.clients._.container-readarr
      den.aspects.infra._.clients._.container-slskd
      den.aspects.infra._.clients._.container-soularr
      den.aspects.infra._.clients._.container-sonarr
      den.aspects.infra._.clients._.container-tempo
      den.aspects.infra._.clients._.container-thelounge
      den.aspects.infra._.clients._.container-tuwunel
      den.aspects.infra._.clients._.container-unifi-exporter
      den.aspects.infra._.clients._.container-unpackerr
      den.aspects.infra._.clients._.container-vpn-gw
      den.aspects.infra._.clients._.container-youtarr
      den.aspects.infra._.clients._.container-zigbee2mqtt
      den.aspects.infra._.clients._.container-zwave
    ];

    nixos =
      { config, lib, ... }:
      let
        cl = config.infra.clients;
        v = config.infra.vlans;
        dom = v.apps.domain;
        gw = v.apps.staticAddresses.gateway;

        promUrl = "http://${cl.container-prometheus.ip}:${toString cl.container-prometheus.ports.default}";

        # Shared context handed to every partial.
        ctx = {
          inherit
            config
            lib
            cl
            v
            dom
            gw
            promUrl
            ;
          # http://<sub>.caddy.<apps-domain>/ — the reverse-proxied web UI url.
          caddy = sub: "http://${sub}.caddy.${dom}/";
          # Direct container endpoint (for widgets/health that bypass Caddy).
          at = ip: port: "http://${ip}:${toString port}";
        };

        settings = import ./_settings.nix ctx;
        widgets = import ./_info.nix ctx;
        bookmarks = import ./_bookmarks.nix ctx;
        services =
          (import ./_overview.nix ctx)
          ++ (import ./_media.nix ctx)
          ++ (import ./_downloads.nix ctx)
          ++ (import ./_automation.nix ctx)
          ++ (import ./_infra.nix ctx)
          ++ (import ./_observability.nix ctx)
          ++ (import ./_network.nix ctx);
      in
      {
        systemd.services."container@homepage" = {
          after = [ "container-wave-arr.target" ];
          before = [ "container-wave-downstream.target" ];
        };

        containerConfigs.homepage = {
          lldpd.enable = true;
          port = cl.container-homepage.ports.default;
          allowedHosts = [
            "homepage.caddy.${dom}"
            "homepage.${dom}"
            cl.container-homepage.ip
          ];
          hostBridge = "br-apps";
          macAddress = cl.container-homepage.mac;
          localAddress = "${cl.container-homepage.ip}/${toString v.apps.subnetPrefix}";
          gateway = gw;
          nameservers = [
            cl.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${cl.container-loki.ip}:${toString cl.container-loki.ports.default}";
            prometheusAddress = "http://${cl.container-prometheus.ip}:${toString cl.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${cl.container-tempo.ip}:${toString cl.container-tempo.ports.otlp-http}/v1/traces";
          };
          envFile = config.sops.templates."homepage-env".path;
          bindMounts = {
            "/run/secrets/homepage-env" = {
              hostPath = config.sops.templates."homepage-env".path;
              isReadOnly = true;
            };
          };
          inherit
            settings
            widgets
            bookmarks
            services
            ;
        };

        containers.homepage.config = {
          systemd.services.homepage-dashboard.serviceConfig.EnvironmentFile = "/run/secrets/homepage-env";
        };
      };
  };
}
