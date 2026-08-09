# pgadmin — host configuration override for pgadmin container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.pgadmin = {
    aspect-name = "hosts.server.containers.pgadmin";
    includes = [
      den.aspects.containers._.pgadmin
      den.aspects.infra._.clients._.container-pgadmin
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.services
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.services
      den.aspects.containers._.postgresql
      den.aspects.infra._.clients._.container-postgres
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.pgadmin;
      in
      {
        containerConfigs.pgadmin = {

          lldpd.enable = true;
          port = config.infra.clients.container-pgadmin.ports.default;
          hostBridge = "br-services";
          macAddress = config.infra.clients.container-pgadmin.mac;
          localAddress = "${config.infra.clients.container-pgadmin.ip}/${toString config.infra.vlans.services.subnetPrefix}";
          gateway = config.infra.vlans.services.staticAddresses.gateway;
          initialPasswordFile = config.sops.secrets.pgadmin-password.path;
          bindMounts = {
            "/var/lib/pgadmin" = {
              hostPath = "/var/lib/appdata/pgadmin";
              isReadOnly = false;
            };
          };
          servers = {
            Servers = {
              "1" = {
                Name = "Central PostgreSQL";
                Group = "Servers";
                Host = config.infra.clients.container-postgres.ip;
                Port = config.infra.clients.container-postgres.ports.default;
                MaintenanceDB = "postgres";
                Username = "postgres";
                SSLMode = "prefer";
              };
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
          "d /var/lib/appdata/pgadmin 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/pgadmin 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@pgadmin" = {
          before = [ "container-wave-core.target" ];
          wants = [ "container@postgresql.service" ];
          after = [
            "container@postgresql.service"
            "container-wave-foundation.target"
          ];
        };
      };
  };
}
