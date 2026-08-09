# gitea — host configuration override for gitea container
{ den, inputs, ... }:
{
  den.aspects.hosts._.server._.containers._.gitea = {
    aspect-name = "hosts.server.containers.gitea";
    includes = [
      den.aspects.containers._.gitea
      den.aspects.infra._.clients._.container-gitea
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.apps
      den.aspects.containers._.postgresql
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.apps
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
      den.aspects.infra._.clients._.container-caddy
    ];

    nixos =
      { config, lib, ... }:
      let
        cfg = config.containerConfigs.gitea;
      in
      {
        containerConfigs.gitea = {

          # Container SSH pubkeys registered on the admin account for git-over-ssh push
          # (committed public keys; private halves live in each container's appdata).
          sshAuthorizedKeys = [
            (lib.removeSuffix "\n" (builtins.readFile (inputs.self + "/pubkeys/code-server.pub")))
            (lib.removeSuffix "\n" (builtins.readFile (inputs.self + "/pubkeys/opencode.pub")))
            (lib.removeSuffix "\n" (builtins.readFile (inputs.self + "/pubkeys/openchamber.pub")))
          ];

          lldpd.enable = true;
          hostBridge = "br-apps";
          macAddress = config.infra.clients.container-gitea.mac;
          localAddress = "${config.infra.clients.container-gitea.ip}/${toString config.infra.vlans.apps.subnetPrefix}";
          gateway = config.infra.vlans.apps.staticAddresses.gateway;
          domain = "gitea.caddy.${config.infra.vlans.apps.domain}";
          rootUrl = "http://gitea.caddy.${config.infra.vlans.apps.domain}/";
          webPort = config.infra.clients.container-gitea.ports.web;
          sshPort = config.infra.clients.container-gitea.ports.ssh;
          dbHost = config.infra.clients.container-postgres.ip;
          dbPort = config.infra.clients.container-postgres.ports.default;
          bindMounts = {
            "/var/lib/gitea" = {
              hostPath = "/var/lib/appdata/gitea";
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
            prometheusInputs = [ "gitea_metrics" ];
            extraSettings = {
              sources.gitea_metrics = {
                type = "prometheus_scrape";
                endpoints = [ "http://localhost:${toString config.containerConfigs.gitea.webPort}/metrics" ];
                scrape_interval_secs = 60;
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/gitea 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/gitea 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@gitea" = {
          before = [ "container-wave-core.target" ];
          wants = [ "container@postgresql.service" ];
          after = [
            "container@postgresql.service"
            "container-wave-foundation.target"
          ];
        };

        containerConfigs.gitea.settings = {
          # gitea's built-in SSH server (git over ssh); off by default in nixpkgs
          server.START_SSH_SERVER = true;
          # SSH clone URLs (repo.ssh_url in the API) are stamped from SSH_DOMAIN, which
          # otherwise falls back to DOMAIN — the caddy web host, which has no SSH. Point
          # it at the container's own reachable name so clone URLs work. Same apps.domain
          # source as `domain`/`rootUrl` above; only the caddy hop differs.
          server.SSH_DOMAIN = "gitea.${config.infra.vlans.apps.domain}";
          metrics = {
            ENABLED = true;
            ENABLE_CONSUMER = true;
          };
          service = {
            ENABLE_REVERSE_PROXY_AUTHENTICATION = true;
            ENABLE_REVERSE_PROXY_AUTO_REGISTRATION = true;
            REVERSE_PROXY_AUTHENTICATION_USER = "X-Webauth-User";
            ENABLE_REVERSE_PROXY_EMAIL = true;
            REVERSE_PROXY_AUTHENTICATION_EMAIL = "X-Webauth-Email";
          };
          security.REVERSE_PROXY_TRUSTED_PROXIES = config.infra.clients.container-caddy.ip;
        };
      };
  };
}
