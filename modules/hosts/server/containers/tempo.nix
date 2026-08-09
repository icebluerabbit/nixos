# modules/hosts/server/containers/tempo.nix
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.tempo = {
    aspect-name = "hosts.server.containers.tempo";
    includes = [
      den.aspects.containers._.tempo
      den.aspects.infra._.clients._.container-tempo
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.services
      den.aspects.hosts._.server._.networking._.vlans._.services
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.tempo;
      in
      {
        systemd.services."container@tempo" = {
          before = [ "container-wave-foundation.target" ];
        };

        containerConfigs.tempo = {
          lldpd.enable = true;
          hostBridge = "br-services";
          macAddress = config.infra.clients.container-tempo.mac;
          localAddress = "${config.infra.clients.container-tempo.ip}/${toString config.infra.vlans.services.subnetPrefix}";
          gateway = config.infra.vlans.services.staticAddresses.gateway;
          port = config.infra.clients.container-tempo.ports.default;
          otlpGrpcPort = config.infra.clients.container-tempo.ports.otlp-grpc;
          otlpHttpPort = config.infra.clients.container-tempo.ports.otlp-http;
          bindMounts = {
            "/var/lib/tempo" = {
              hostPath = "/var/lib/appdata/tempo";
              isReadOnly = false;
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
            lokiInputs = [ "filter_tempo_noise" ];
            extraSettings.transforms.filter_tempo_noise = {
              type = "filter";
              inputs = [ "journal" ];
              condition = ''!contains(string!(.message), "no jobs found")'';
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/tempo 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/tempo 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];
      };
  };
}
