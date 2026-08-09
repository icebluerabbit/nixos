# modules/hosts/server/containers/prometheus.nix
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.prometheus = {
    aspect-name = "hosts.server.containers.prometheus";
    includes = [
      den.aspects.containers._.prometheus
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.services
      den.aspects.hosts._.server._.networking._.vlans._.services
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.prometheus;
      in
      {
        systemd.services."container@prometheus" = {
          before = [ "container-wave-foundation.target" ];
        };

        containerConfigs.prometheus = {

          lldpd.enable = true;
          hostBridge = "br-services";
          macAddress = config.infra.clients.container-prometheus.mac;
          port = config.infra.clients.container-prometheus.ports.default;
          localAddress = "${config.infra.clients.container-prometheus.ip}/${toString config.infra.vlans.services.subnetPrefix}";
          gateway = config.infra.vlans.services.staticAddresses.gateway;
          bindMounts = {
            # /var/lib/prometheus2, not /var/lib/prometheus: services.prometheus
            # defaults stateDir to "prometheus2", so the TSDB is at
            # /var/lib/prometheus2/data. Mounting the un-suffixed path left the
            # database inside the ephemeral container root, where every restart
            # discarded it — retention was however long the container had been up.
            "/var/lib/prometheus2" = {
              hostPath = "/var/lib/appdata/prometheus";
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
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/prometheus 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/prometheus 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];
      };
  };
}
