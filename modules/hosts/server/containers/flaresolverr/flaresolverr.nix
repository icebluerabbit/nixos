# flaresolverr — host configuration override for flaresolverr container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.flaresolverr = {
    aspect-name = "hosts.server.containers.flaresolverr";
    includes = [
      den.aspects.containers._.flaresolverr
      den.aspects.infra._.clients._.container-flaresolverr
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.media
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        exporter = import ./_exporter.nix;
      in
      {
        systemd.services."container@flaresolverr" = {
          after = [ "container-wave-core.target" ];
          before = [ "container-wave-apps.target" ];
        };

        containerConfigs.flaresolverr = {
          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-flaresolverr.mac;
          localAddress = "${config.infra.clients.container-flaresolverr.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-flaresolverr.ports.default;
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            inherit (exporter) prometheusInputs extraSettings;
          };
        };
      };
  };
}
