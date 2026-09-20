# postgresql — host configuration override for postgresql database container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.postgresql = {
    aspect-name = "hosts.server.containers.postgresql";
    includes = [
      den.aspects.containers._.postgresql
      den.aspects.infra._.clients._.container-postgres
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.services
      den.aspects.hosts._.server._.networking._.vlans._.services
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo

      # Database client containers
      den.aspects.containers._.pgadmin
      den.aspects.infra._.clients._.container-pgadmin
      den.aspects.containers._.radarr
      den.aspects.infra._.clients._.container-radarr
      den.aspects.containers._.lidarr
      den.aspects.infra._.clients._.container-lidarr
      den.aspects.containers._.readarr
      den.aspects.infra._.clients._.container-readarr
      den.aspects.containers._.sonarr
      den.aspects.infra._.clients._.container-sonarr
      den.aspects.containers._.bazarr
      den.aspects.infra._.clients._.container-bazarr
      den.aspects.containers._.prowlarr
      den.aspects.infra._.clients._.container-prowlarr
      den.aspects.containers._.gitea
      den.aspects.infra._.clients._.container-gitea
      den.aspects.containers._.home-assistant
      den.aspects.infra._.clients._.container-home-assistant
      den.aspects.containers._.grafana
      den.aspects.infra._.clients._.container-grafana
      den.aspects.containers._.authelia
      den.aspects.infra._.clients._.container-authelia
      den.aspects.containers._.lldap
      den.aspects.infra._.clients._.container-lldap
      den.aspects.containers._.managarr
      den.aspects.infra._.clients._.container-managarr
    ];

    nixos =
      { config, ... }:
      {
        systemd.services."container@postgresql" = {
          before = [ "container-wave-foundation.target" ];
        };

        containerConfigs.postgresql = {

          lldpd.enable = true;
          hostBridge = "br-services";
          macAddress = config.infra.clients.container-postgres.mac;
          localAddress = "${config.infra.clients.container-postgres.ip}/${toString config.infra.vlans.services.subnetPrefix}";
          gateway = config.infra.vlans.services.staticAddresses.gateway;
          bindMounts = {
            "/var/lib/postgresql" = {
              hostPath = "/var/lib/appdata/postgresql";
              isReadOnly = false;
            };
          };
          auth = [
            {
              database = "all";
              user = "all";
              ip = config.infra.clients.container-pgadmin.ip;
            }
            {
              database = "all";
              user = config.containerConfigs.radarr.dbUser;
              ip = config.infra.clients.container-radarr.ip;
            }
            {
              database = "all";
              user = config.containerConfigs.lidarr.dbUser;
              ip = config.infra.clients.container-lidarr.ip;
            }
            {
              database = "all";
              user = config.containerConfigs.readarr.dbUser;
              ip = config.infra.clients.container-readarr.ip;
            }
            {
              database = "all";
              user = config.containerConfigs.sonarr.dbUser;
              ip = config.infra.clients.container-sonarr.ip;
            }
            {
              database = "all";
              user = config.containerConfigs.bazarr.dbUser;
              ip = config.infra.clients.container-bazarr.ip;
            }
            {
              database = "all";
              user = config.containerConfigs.prowlarr.dbUser;
              ip = config.infra.clients.container-prowlarr.ip;
            }
            {
              database = "all";
              user = config.containerConfigs.gitea.dbUser;
              ip = config.infra.clients.container-gitea.ip;
            }
            {
              database = "all";
              user = config.containerConfigs.home-assistant.dbUser;
              ip = config.infra.clients.container-home-assistant.ip;
            }
            {
              database = "all";
              user = config.containerConfigs.grafana.dbUser;
              ip = config.infra.clients.container-grafana.ip;
            }
            {
              database = "all";
              user = config.containerConfigs.authelia.dbUser;
              ip = config.infra.clients.container-authelia.ip;
            }
            {
              database = "all";
              user = config.containerConfigs.lldap.dbUser;
              ip = config.infra.clients.container-lldap.ip;
            }
            {
              database = "all";
              user = config.containerConfigs.managarr.dbUser;
              ip = config.infra.clients.container-managarr.ip;
            }
          ];
          ensureUsers = [
            {
              name = config.containerConfigs.radarr.dbUser;
              ensureDBOwnership = true;
            }
            {
              name = config.containerConfigs.lidarr.dbUser;
              ensureDBOwnership = true;
            }
            {
              name = config.containerConfigs.readarr.dbUser;
              ensureDBOwnership = true;
            }
            {
              name = config.containerConfigs.sonarr.dbUser;
              ensureDBOwnership = true;
            }
            {
              name = config.containerConfigs.bazarr.dbUser;
              ensureDBOwnership = true;
            }
            {
              name = config.containerConfigs.prowlarr.dbUser;
              ensureDBOwnership = true;
            }
            {
              name = config.containerConfigs.gitea.dbUser;
              ensureDBOwnership = true;
            }
            {
              name = config.containerConfigs.home-assistant.dbUser;
              ensureDBOwnership = true;
            }
            {
              name = config.containerConfigs.grafana.dbUser;
              ensureDBOwnership = true;
            }
            {
              name = config.containerConfigs.authelia.dbUser;
              ensureDBOwnership = true;
            }
            {
              name = config.containerConfigs.lldap.dbUser;
              ensureDBOwnership = true;
            }
            {
              name = config.containerConfigs.managarr.dbUser;
              ensureDBOwnership = true;
            }
          ];
          ensureDatabases = [
            config.containerConfigs.radarr.dbName
            config.containerConfigs.radarr.dbLogName
            config.containerConfigs.lidarr.dbName
            config.containerConfigs.lidarr.dbLogName
            config.containerConfigs.readarr.dbName
            config.containerConfigs.readarr.dbLogName
            config.containerConfigs.readarr.dbCacheName
            config.containerConfigs.sonarr.dbName
            config.containerConfigs.sonarr.dbLogName
            config.containerConfigs.bazarr.dbName
            config.containerConfigs.prowlarr.dbName
            config.containerConfigs.prowlarr.dbLogName
            config.containerConfigs.gitea.dbName
            config.containerConfigs.home-assistant.dbName
            config.containerConfigs.grafana.dbName
            config.containerConfigs.authelia.dbName
            config.containerConfigs.lldap.dbName
            config.containerConfigs.managarr.dbName
          ];
          dbOwnerships = {
            "${config.containerConfigs.radarr.dbName}" = config.containerConfigs.radarr.dbUser;
            "${config.containerConfigs.radarr.dbLogName}" = config.containerConfigs.radarr.dbUser;
            "${config.containerConfigs.lidarr.dbName}" = config.containerConfigs.lidarr.dbUser;
            "${config.containerConfigs.lidarr.dbLogName}" = config.containerConfigs.lidarr.dbUser;
            "${config.containerConfigs.readarr.dbName}" = config.containerConfigs.readarr.dbUser;
            "${config.containerConfigs.readarr.dbLogName}" = config.containerConfigs.readarr.dbUser;
            "${config.containerConfigs.readarr.dbCacheName}" = config.containerConfigs.readarr.dbUser;
            "${config.containerConfigs.sonarr.dbName}" = config.containerConfigs.sonarr.dbUser;
            "${config.containerConfigs.sonarr.dbLogName}" = config.containerConfigs.sonarr.dbUser;
            "${config.containerConfigs.bazarr.dbName}" = config.containerConfigs.bazarr.dbUser;
            "${config.containerConfigs.prowlarr.dbName}" = config.containerConfigs.prowlarr.dbUser;
            "${config.containerConfigs.prowlarr.dbLogName}" = config.containerConfigs.prowlarr.dbUser;
            "${config.containerConfigs.gitea.dbName}" = config.containerConfigs.gitea.dbUser;
            "${config.containerConfigs.home-assistant.dbName}" = config.containerConfigs.home-assistant.dbUser;
            "${config.containerConfigs.grafana.dbName}" = config.containerConfigs.grafana.dbUser;
            "${config.containerConfigs.authelia.dbName}" = config.containerConfigs.authelia.dbUser;
            "${config.containerConfigs.lldap.dbName}" = config.containerConfigs.lldap.dbUser;
            "${config.containerConfigs.managarr.dbName}" = config.containerConfigs.managarr.dbUser;
          };
          # The *arr apps run their own daily VACUUM housekeeping. As plain database
          # owners that walks the shared catalogs (pg_authid, pg_database, …) they have
          # no rights to, emitting ~11 "permission denied to vacuum" WARNINGs per run.
          # pg_maintain lets those succeed instead of warning. Only the roles that
          # actually self-vacuum are listed.
          maintainRoles = [
            config.containerConfigs.radarr.dbUser
            config.containerConfigs.lidarr.dbUser
            config.containerConfigs.readarr.dbUser
            config.containerConfigs.sonarr.dbUser
            config.containerConfigs.prowlarr.dbUser
          ];
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          port = config.infra.clients.container-postgres.ports.default;
          exporterPort = config.infra.clients.container-postgres.ports.exporter;
          extraSettings = {
            shared_buffers = "16GB";
            effective_cache_size = "60GB";
            work_mem = "16MB";
            maintenance_work_mem = "2GB";
            wal_buffers = "64MB";
            random_page_cost = 1.1;
            effective_io_concurrency = 200;
            max_worker_processes = 32;
            max_parallel_workers = 16;
            max_parallel_workers_per_gather = 4;
          };
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            prometheusInputs = [ "pg_exporter" ];
            extraSettings = {
              sources.pg_exporter = {
                type = "prometheus_scrape";
                endpoints = [
                  "http://localhost:${toString config.containerConfigs.postgresql.exporterPort}/metrics"
                ];
                scrape_interval_secs = 60;
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/postgresql 0700 71 71 - -"
          "Z /var/lib/appdata/postgresql 0700 71 71 - -"
          # nodatacow (+C) on the data dir: Postgres does random in-page rewrites, which on
          # CoW btrfs relocate every touched extent → heavy fragmentation, write
          # amplification, and extent-tree bloat. +C makes new files overwrite in place.
          # Only affects files created AFTER this is set; the existing cluster must be
          # recreated inside the dir (dump/restore or stop+copy) to inherit it.
          "h /var/lib/appdata/postgresql - - - - +C"
        ];
      };
  };
}
