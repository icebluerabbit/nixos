# modules/hosts/server/containers/zwave.nix
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.zwave = {
    aspect-name = "hosts.server.containers.zwave";
    includes = [
      den.aspects.containers._.zwave
      den.aspects.infra._.clients._.container-zwave
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.iot
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.iot
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.zwave;
      in
      {
        systemd.services."container@zwave" = {
          after = [ "container-wave-foundation.target" ];
          before = [ "container-wave-core.target" ];
        };

        containerConfigs.zwave = {
          lldpd.enable = true;
          hostBridge = "br-iot";
          macAddress = config.infra.clients.container-zwave.mac;
          localAddress = "${config.infra.clients.container-zwave.ip}/${toString config.infra.vlans.iot.subnetPrefix}";
          gateway = config.infra.vlans.iot.staticAddresses.gateway;
          uiPort = config.infra.clients.container-zwave.ports.ui;
          wsPort = config.infra.clients.container-zwave.ports.ws;
          idVendor = "303a";
          idProduct = "4001";
          serialPort = "/dev/zwave";
          bindMounts = {
            "/var/lib/zwave-js-ui" = {
              hostPath = "/var/lib/appdata/zwave-js-ui";
              isReadOnly = false;
            };
            "/dev/zwave" = {
              hostPath = "/dev/zwave";
              isReadOnly = false;
            };
          };
          envFile = config.sops.templates."zwave-env".path;
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            prometheusInputs = [ "zwave_prom" ];
            extraSettings = {
              sources.zwave_prom = {
                type = "prometheus_scrape";
                endpoints = [ "http://localhost:8091/metrics" ];
                scrape_interval_secs = 15;
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/zwave-js-ui 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/zwave-js-ui 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];
      };
  };
}
