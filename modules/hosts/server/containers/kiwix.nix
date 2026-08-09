# kiwix — host configuration override for kiwix container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.kiwix = {
    aspect-name = "hosts.server.containers.kiwix";
    includes = [
      den.aspects.containers._.kiwix
      den.aspects.infra._.clients._.container-kiwix
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.apps
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.apps
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.kiwix;
      in
      {
        containerConfigs.kiwix = {

          lldpd.enable = true;
          port = config.infra.clients.container-kiwix.ports.http;
          hostBridge = "br-apps";
          macAddress = config.infra.clients.container-kiwix.mac;
          localAddress = "${config.infra.clients.container-kiwix.ip}/${toString config.infra.vlans.apps.subnetPrefix}";
          gateway = config.infra.vlans.apps.staticAddresses.gateway;
          bindMounts = {
            "/var/lib/kiwix" = {
              hostPath = "/var/lib/appdata/kiwix";
              isReadOnly = false;
            };
            "/zim" = {
              hostPath = "/mnt/data/kiwix";
              isReadOnly = false;
            };
          };
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          sync = {
            enable = true;
            books = [
              {
                category = "wikipedia";
                prefix = "wikipedia_en_all_maxi";
              }
              {
                category = "wiktionary";
                prefix = "wiktionary_en_all_nopic";
              }
            ];
          };
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
          };
        };

        systemd.tmpfiles.rules = [
          # "d /mnt/data/kiwix 0775 ${toString cfg.uid} ${toString cfg.gid} - -"
          # "Z /mnt/data/kiwix 0775 ${toString cfg.uid} ${toString cfg.gid} - -"
          "d /var/lib/appdata/kiwix 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/kiwix 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "d /mnt/data/kiwix - - - - -"
        ];

        systemd.services."container@kiwix" = {
          before = [ "container-wave-apps.target" ];
          after = [
            "remote-fs.target"
            "container-wave-core.target"
          ];
        };
      };
  };
}
