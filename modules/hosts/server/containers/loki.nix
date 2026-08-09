# modules/hosts/server/containers/loki.nix
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.loki = {
    aspect-name = "hosts.server.containers.loki";
    includes = [
      den.aspects.containers._.loki
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.services
      den.aspects.hosts._.server._.networking._.vlans._.services
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.loki;
      in
      {
        systemd.services."container@loki" = {
          before = [ "container-wave-foundation.target" ];
        };

        containerConfigs.loki = {

          lldpd.enable = true;
          hostBridge = "br-services";
          macAddress = config.infra.clients.container-loki.mac;
          port = config.infra.clients.container-loki.ports.default;
          localAddress = "${config.infra.clients.container-loki.ip}/${toString config.infra.vlans.services.subnetPrefix}";
          gateway = config.infra.vlans.services.staticAddresses.gateway;
          bindMounts = {
            "/var/lib/loki" = {
              hostPath = "/var/lib/appdata/loki";
              isReadOnly = false;
            };
          };
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = null;
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            prometheusInputs = [ "loki_metrics" ];
            extraSettings = {
              sources.loki_metrics = {
                type = "prometheus_scrape";
                endpoints = [ "http://localhost:${toString config.containerConfigs.loki.port}/metrics" ];
                scrape_interval_secs = 60;
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/loki 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/loki 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];
      };
  };
}
