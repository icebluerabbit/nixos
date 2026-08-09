# authelia — host configuration override for authelia forward auth container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.authelia = {
    aspect-name = "hosts.server.containers.authelia";
    includes = [
      den.aspects.containers._.authelia
      den.aspects.infra._.clients._.container-authelia
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.services
      den.aspects.infra._.networks._.apps
      den.aspects.hosts._.server._.networking._.vlans._.services
      den.aspects.hosts._.server._.security
      den.aspects.containers._.postgresql
      den.aspects.infra._.clients._.container-postgres
      den.aspects.infra._.clients._.container-lldap
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      let
        cfg = config.containerConfigs.authelia;
      in
      {
        containerConfigs.authelia = {
          lldpd.enable = true;
          hostBridge = "br-services";
          macAddress = config.infra.clients.container-authelia.mac;
          localAddress = "${config.infra.clients.container-authelia.ip}/${toString config.infra.vlans.services.subnetPrefix}";
          gateway = config.infra.vlans.services.staticAddresses.gateway;
          port = config.infra.clients.container-authelia.ports.default;
          metricsPort = config.infra.clients.container-authelia.ports.metrics;
          dbHost = config.infra.clients.container-postgres.ip;
          dbPort = config.infra.clients.container-postgres.ports.default;
          autheliaUrl = "https://authelia.caddy.${config.infra.vlans.apps.domain}";
          sessionDomain = "caddy.${config.infra.vlans.apps.domain}";
          jwtSecretFile = "/run/secrets/authelia-jwt-secret";
          storageEncryptionKeyFile = "/run/secrets/authelia-storage-encryption-key";
          oidcHmacSecretFile = "/run/secrets/authelia-oidc-hmac-secret";
          oidcIssuerPrivateKeyFile = "/run/secrets/authelia-oidc-jwk-rsa-key";
          ldapAddress = "ldap://${config.infra.clients.container-lldap.ip}:${toString config.infra.clients.container-lldap.ports.ldap}";
          ldapBaseDN = "dc=homelab";
          ldapUser = "uid=authelia,ou=people,dc=homelab";
          ldapPasswordFile = "/run/secrets/authelia-ldap-password";
          oidcClients = [
            {
              client_id = "homeassistant";
              client_name = "Home Assistant";
              public = true;
              require_pkce = true;
              pkce_challenge_method = "S256";
              consent_mode = "implicit";
              authorization_policy = "one_factor";
              scopes = [
                "openid"
                "profile"
                "groups"
              ];
              redirect_uris = [
                "https://home-assistant.caddy.${config.infra.vlans.apps.domain}/auth/oidc/callback"
              ];
            }
          ];
          accessRules = [
            {
              domain = "*.caddy.${config.infra.vlans.apps.domain}";
              policy = "one_factor";
            }
          ];
          bindMounts = {
            "/var/lib/authelia" = {
              hostPath = "/var/lib/appdata/authelia";
              isReadOnly = false;
            };
            "/run/secrets/authelia-jwt-secret" = {
              hostPath = config.sops.secrets."authelia-jwt-secret".path;
              isReadOnly = true;
            };
            "/run/secrets/authelia-storage-encryption-key" = {
              hostPath = config.sops.secrets."authelia-storage-encryption-key".path;
              isReadOnly = true;
            };
            "/run/secrets/authelia-oidc-hmac-secret" = {
              hostPath = config.sops.secrets."authelia-oidc-hmac-secret".path;
              isReadOnly = true;
            };
            "/run/secrets/authelia-oidc-jwk-rsa-key" = {
              hostPath = config.sops.secrets."authelia-oidc-jwk-rsa-key".path;
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
            prometheusInputs = [ "authelia_metrics" ];
            extraSettings.sources.authelia_metrics = {
              type = "prometheus_scrape";
              endpoints = [
                "http://localhost:${toString config.infra.clients.container-authelia.ports.metrics}/"
              ];
              scrape_interval_secs = 60;
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/authelia 0750 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/authelia 0750 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@authelia" = {
          before = [ "container-wave-core.target" ];
          wants = [
            "container@postgresql.service"
            "container@lldap.service"
          ];
          after = [
            "container@postgresql.service"
            "container@lldap.service"
            "container-wave-foundation.target"
          ];
        };
      };
  };
}
