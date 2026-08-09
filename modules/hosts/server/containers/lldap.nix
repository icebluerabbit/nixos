# lldap — host configuration override for lldap container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.lldap = {
    aspect-name = "hosts.server.containers.lldap";
    includes = [
      den.aspects.containers._.lldap
      den.aspects.infra._.clients._.container-lldap
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.clients._.container-postgres
      den.aspects.infra._.networks._.services
      den.aspects.infra._.networks._.apps
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.services
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.lldap;
      in
      {
        containerConfigs.lldap = {
          lldpd.enable = true;
          hostBridge = "br-services";
          macAddress = config.infra.clients.container-lldap.mac;
          localAddress = "${config.infra.clients.container-lldap.ip}/${toString config.infra.vlans.services.subnetPrefix}";
          gateway = config.infra.vlans.services.staticAddresses.gateway;
          ldapPort = config.infra.clients.container-lldap.ports.ldap;
          httpPort = config.infra.clients.container-lldap.ports.http;
          baseDN = "dc=homelab";
          httpUrl = "http://lldap.caddy.${config.infra.vlans.apps.domain}";
          dbHost = config.infra.clients.container-postgres.ip;
          dbPort = config.infra.clients.container-postgres.ports.default;
          jwtSecretPath = "/run/secrets/lldap-jwt-secret";
          adminPassPath = "/run/secrets/lldap-admin-password";
          seedUsers = [
            {
              id = "authelia";
              email = "authelia@homelab";
              displayName = "Authelia Bind";
              groups = [ "lldap_strict_readonly" ];
              passwordPath = "/run/secrets/authelia-ldap-password";
            }
          ];
          bindMounts = {
            "/var/lib/lldap" = {
              hostPath = "/var/lib/appdata/lldap";
              isReadOnly = false;
            };
            "/run/secrets/lldap-jwt-secret" = {
              hostPath = config.sops.secrets."lldap-jwt-secret".path;
              isReadOnly = true;
            };
            "/run/secrets/lldap-admin-password" = {
              hostPath = config.sops.secrets."lldap-admin-password".path;
              isReadOnly = true;
            };
            "/run/secrets/authelia-ldap-password" = {
              hostPath = config.sops.secrets."authelia-ldap-password".path;
              isReadOnly = true;
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
          "d /var/lib/appdata/lldap 0750 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/lldap 0750 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@lldap" = {
          before = [ "container-wave-foundation.target" ];
          wants = [ "container@postgresql.service" ];
          after = [ "container@postgresql.service" ];
        };
      };
  };
}
