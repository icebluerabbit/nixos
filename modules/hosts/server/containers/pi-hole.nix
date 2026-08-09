# modules/hosts/server/containers/pi-hole.nix
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.pi-hole = {
    aspect-name = "hosts.server.containers.pi-hole";
    includes = [
      den.aspects.containers._.pi-hole
      den.aspects.infra._.clients._.container-pi-hole
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.services
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.services
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.pi-hole;
      in
      {
        systemd.services."container@pi-hole" = {
          before = [ "container-wave-foundation.target" ];
        };

        containerConfigs.pi-hole = {

          lldpd.enable = true;
          dnsPort = config.infra.clients.container-pi-hole.ports.dns;
          webPort = config.infra.clients.container-pi-hole.ports.web;
          hostBridge = "br-services";
          macAddress = config.infra.clients.container-pi-hole.mac;
          localAddress = "${config.infra.clients.container-pi-hole.ip}/${toString config.infra.vlans.services.subnetPrefix}";
          gateway = config.infra.vlans.services.staticAddresses.gateway;
          hostName = "pi-hole.server.internal";
          bindMounts = {
            "/var/lib/pihole" = {
              hostPath = "/var/lib/appdata/pihole";
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
          "d /var/lib/appdata/pihole 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/pihole 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];
      };
  };
}
