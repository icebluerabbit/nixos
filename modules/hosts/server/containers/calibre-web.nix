# calibre-web — host configuration override for calibre-web container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.calibre-web = {
    aspect-name = "hosts.server.containers.calibre-web";
    includes = [
      den.aspects.containers._.calibre-web
      den.aspects.infra._.clients._.container-calibre-web
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.media
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.calibre-web;
      in
      {
        containerConfigs.calibre-web = {

          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-calibre-web.mac;
          localAddress = "${config.infra.clients.container-calibre-web.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-calibre-web.ports.default;

          libraryDir = "/mnt/media/library/books";
          ingestDir = "/mnt/media/library/cwa-ingest";

          bindMounts = {
            "/var/lib/calibre-web" = {
              hostPath = "/var/lib/appdata/calibre-web";
              isReadOnly = false;
            };
            "/mnt/media" = {
              hostPath = "/mnt/media";
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
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/calibre-web 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/calibre-web 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@calibre-web" = {
          before = [ "container-wave-downstream.target" ];
          after = [
            "remote-fs.target"
            "container-wave-arr.target"
          ];
        };
      };
  };
}
