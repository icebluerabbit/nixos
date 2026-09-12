# stashtrap — host configuration override for stashtrap container
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.stashtrap = {
    aspect-name = "hosts.server.containers.stashtrap";
    includes = [
      den.aspects.containers._.stashtrap
      den.aspects.infra._.clients._.container-stashtrap
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
        cfg = config.containerConfigs.stashtrap;
      in
      {
        containerConfigs.stashtrap = {

          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-stashtrap.mac;
          localAddress = "${config.infra.clients.container-stashtrap.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-stashtrap.ports.default;
          metricsPort = config.infra.clients.container-stashtrap.ports.metrics;
          # Same account Stash's stash-box connection authenticates with.
          stashboxKeyFile = config.sops.secrets.stashdb-apikey.path;
          # Favourites change rarely; the scene crawl behind them is the
          # expensive part, so nightly rather than hourly.
          syncInterval = "daily";
          # OnlyFans rips can never match the catalogue — the platform is an
          # alias of seventy-five studios and the performer is a handle — so
          # they are taken on the platform alone. 5.8% of BitPorn's output,
          # roughly 40 releases a day, which is worth having while downloads
          # are free and worth reconsidering when they are not.
          platforms = [ "onlyfans" ];
          bindMounts = {
            # SQLite on local disk. NFS would be a locking hazard, and the
            # corpus is small enough that it never wants the array.
            "/var/lib/stashtrap" = {
              hostPath = "/var/lib/appdata/stashtrap";
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
            prometheusInputs = [ "stashtrap_metrics" ];
            extraSettings = {
              sources.stashtrap_metrics = {
                type = "prometheus_scrape";
                endpoints = [
                  "http://127.0.0.1:${toString config.infra.clients.container-stashtrap.ports.metrics}/metrics"
                ];
                scrape_interval_secs = 30;
              };
            };
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/stashtrap 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/stashtrap 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];

        systemd.services."container@stashtrap" = {
          before = [ "container-wave-downstream.target" ];
          after = [ "container-wave-apps.target" ];
        };
      };
  };
}
