# modules/hosts/server/containers/pi-hole.nix
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.pi-hole = {
    aspect-name = "hosts.server.containers.pi-hole";
    includes = [
      den.aspects.containers._.pi-hole
      den.aspects.infra._.clients._.container-pi-hole
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.services
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.services
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
      den.aspects.infra._.clients._.container-mosquitto

      # VLAN networks (source of truth for the domains forwarded to dnsmasq)
      den.aspects.infra._.networks._.apps
      den.aspects.infra._.networks._.default
      den.aspects.infra._.networks._.dmz
      den.aspects.infra._.networks._.download
      den.aspects.infra._.networks._.game
      den.aspects.infra._.networks._.guest
      den.aspects.infra._.networks._.iot
      den.aspects.infra._.networks._.media
      den.aspects.infra._.networks._.mgmt
      den.aspects.infra._.networks._.storage
      den.aspects.infra._.networks._.trust
      den.aspects.infra._.networks._.work
    ];

    nixos =
      { config, lib, ... }:
      let
        cfg = config.containerConfigs.pi-hole;
      in
      {
        systemd.services."container@pi-hole" = {
          before = [ "container-wave-foundation.target" ];
        };

        containerConfigs.pi-hole = {

          lldpd.enable = true;
          dnsPort = config.infra.clients.container-pi-hole.ports.dns;
          webPort = config.infra.clients.container-pi-hole.ports.web;
          hostBridge = "br-services";
          macAddress = config.infra.clients.container-pi-hole.mac;
          localAddress = "${config.infra.clients.container-pi-hole.ip}/${toString config.infra.vlans.services.subnetPrefix}";
          gateway = config.infra.vlans.services.staticAddresses.gateway;
          hostName = "pi-hole.server.internal";
          bindMounts = {
            "/var/lib/pihole" = {
              hostPath = "/var/lib/appdata/pihole";
              isReadOnly = false;
            };
          };
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];

          upstreams = [
            "1.1.1.1"
            "1.0.0.1"
          ];
          # Clients live on trust (desktop) and 192.168.4.0/24 (VPN), never on
          # Pi-hole's own subnet, so the LOCAL default would drop every query.
          listeningMode = "ALL";
          internalResolver = config.infra.clients.container-dns.ip;
          internalDomains = lib.mapAttrsToList (_: vlan: vlan.domain) config.infra.vlans;
          reverseZones = [ "10.0.0.0/8" ];
          maxDBdays = 30;
          lists = [
            {
              url = "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts";
              type = "block";
              description = "StevenBlack unified hosts";
              enabled = true;
            }
          ];
          exporter = {
            enable = true;
            port = config.infra.clients.container-pi-hole.ports.exporter;
          };

          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            prometheusInputs = [ "pihole_exporter" ];
            extraSettings = {
              sources.pihole_exporter = {
                type = "prometheus_scrape";
                endpoints = [
                  "http://127.0.0.1:${toString config.infra.clients.container-pi-hole.ports.exporter}/metrics"
                ];
                scrape_interval_secs = 30;
              };
              # Lifecycle events only — never queries. Pi-hole logs every lookup, so
              # anything query-shaped here would post thousands of Matrix messages a
              # day. This keeps the four transitions worth being told about and aborts
              # the rest; the numbers behind them come from Prometheus at send time,
              # not from parsing gravity's stdout.
              transforms.dns_events = {
                type = "remap";
                inputs = [ "journal" ];
                drop_on_abort = true;
                drop_on_error = true;
                source = ''
                  # `UNIT`, not `_SYSTEMD_UNIT`. Every transition below is logged by
                  # systemd itself, so the writing process is PID 1 and _SYSTEMD_UNIT
                  # reads "init.scope" — the unit being talked about is only in UNIT.
                  # _SYSTEMD_UNIT is right for a service's own stdout, which is why
                  # the fallback stays.
                  unit = if exists(.UNIT) {
                    to_string(.UNIT) ?? ""
                  } else {
                    to_string(._SYSTEMD_UNIT) ?? ""
                  }
                  msg = to_string(.message) ?? ""
                  kind = ""
                  if unit == "pihole-gravity-refresh.service" {
                    if starts_with(msg, "Finished") { kind = "gravity_refreshed" }
                    if match(msg, r'Failed with result') { kind = "gravity_failed" }
                  }
                  if unit == "pihole-ftl.service" {
                    if match(msg, r'Failed with result') { kind = "ftl_failed" }
                    if match(msg, r'Scheduled restart') { kind = "ftl_restarted" }
                  }
                  if kind == "" { abort }
                  . = { "kind": kind, "unit": unit, "message": msg, "host": "pi-hole" }
                '';
              };
              sinks.dns_events_mqtt = {
                type = "mqtt";
                inputs = [ "dns_events" ];
                host = config.infra.clients.container-mosquitto.ip;
                port = config.infra.clients.container-mosquitto.ports.default;
                client_id = "vector-pi-hole"; # unique per container → no broker client-id collision
                topic = "homelab/notify/dns/events/{{ kind }}";
                quality_of_service = "atleastonce";
                retain = false;
                encoding.codec = "json";
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/pihole 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/pihole 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];
      };
  };
}
