# qui — host configuration override for qui container (replaces flood)
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.qui = {
    aspect-name = "hosts.server.containers.qui";
    includes = [
      den.aspects.containers._.qui
      den.aspects.infra._.clients._.container-qui
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.apps
      den.aspects.infra._.networks._.trust
      den.aspects.hosts._.server._.networking._.vlans._.apps
      den.aspects.hosts._.server._.security
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.qui;
      in
      {
        systemd.services."container@qui" = {
          after = [ "container-wave-apps.target" ];
          before = [ "container-wave-arr.target" ];
        };

        containerConfigs.qui = {
          lldpd.enable = true;
          hostBridge = "br-apps";
          macAddress = config.infra.clients.container-qui.mac;
          localAddress = "${config.infra.clients.container-qui.ip}/${toString config.infra.vlans.apps.subnetPrefix}";
          gateway = config.infra.vlans.apps.staticAddresses.gateway;
          port = config.infra.clients.container-qui.ports.default;
          secretFile = "/run/secrets/qui-session-secret";
          # Auto-registered qBittorrent instance (qui has no declarative instance
          # config → qui_provision.py POSTs it via the API at startup).
          qbtUrl = "http://${config.infra.clients.container-qbittorrent.ip}:${toString config.infra.clients.container-qbittorrent.ports.webui}";
          # Auth is disabled → access is gated purely by source network. The
          # allowlist is checked against the DIRECT peer (before RealIP), so:
          #   127.0.0.1  — the in-container qui_provision.py call
          #   apps       — Caddy is the peer for all proxied UI traffic
          #   trust      — direct-to-IP access from user devices
          allowedCIDRs = [
            "127.0.0.1/32"
            "${config.infra.vlans.apps.subnet}/${toString config.infra.vlans.apps.subnetPrefix}"
            "${config.infra.vlans.trust.subnet}/${toString config.infra.vlans.trust.subnetPrefix}"
          ];
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
          };
          bindMounts = {
            "/var/lib/qui" = {
              hostPath = "/var/lib/appdata/qui";
              isReadOnly = false;
            };
            "/run/secrets/qui-session-secret" = {
              hostPath = config.sops.secrets."qui-session-secret".path;
              isReadOnly = true;
            };
          };
        };

        # openssl rand -hex 32 → add the value via `just sops-host-secrets`.
        sops.secrets."qui-session-secret" = { };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/qui 0750 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/qui 0750 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];
      };
  };
}
