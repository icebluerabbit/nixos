# cinny — host configuration override for cinny Matrix web client container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.cinny = {
    aspect-name = "hosts.server.containers.cinny";
    includes = [
      den.aspects.containers._.cinny
      den.aspects.infra._.clients._.container-cinny
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.apps
      den.aspects.hosts._.server._.networking._.vlans._.apps
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      {
        systemd.services."container@cinny" = {
          after = [ "container-wave-core.target" ];
          before = [ "container-wave-apps.target" ];
        };

        containerConfigs.cinny = {
          lldpd.enable = true;
          hostBridge = "br-apps";
          macAddress = config.infra.clients.container-cinny.mac;
          localAddress = "${config.infra.clients.container-cinny.ip}/${toString config.infra.vlans.apps.subnetPrefix}";
          gateway = config.infra.vlans.apps.staticAddresses.gateway;
          port = config.infra.clients.container-cinny.ports.default;
          homeserverDomain = "tuwunel.caddy.${config.infra.vlans.apps.domain}";
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
        };
      };
  };
}
