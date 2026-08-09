# readarr — host configuration override for readarr container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.readarr = {
    aspect-name = "hosts.server.containers.readarr";
    includes = [
      den.aspects.containers._.readarr
      den.aspects.infra._.clients._.container-readarr
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.media
      den.aspects.containers._.postgresql
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.readarr;
      in
      {
        containerConfigs.readarr = {

          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-readarr.mac;
          localAddress = "${config.infra.clients.container-readarr.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-readarr.ports.default;
          dbHost = config.infra.clients.container-postgres.ip;
          dbPort = config.infra.clients.container-postgres.ports.default;
          bindMounts = {
            "/var/lib/readarr" = {
              hostPath = "/var/lib/appdata/readarr";
              isReadOnly = false;
            };
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
          };
          envFile = config.sops.templates."readarr-env".path;
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

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/readarr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/readarr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@readarr" = {
          before = [ "container-wave-arr.target" ];
          wants = [ "container@postgresql.service" ];
          after = [
            "container@postgresql.service"
            "remote-fs.target"
            "container-wave-apps.target"
          ];
        };
      };
  };
}
