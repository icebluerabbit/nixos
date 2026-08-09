# modules/logging/vector.nix
{ den, ... }:
{
  den.aspects.logging._.vector = {
    aspect-name = "logging.vector";
    includes = [
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, lib, ... }:
      {
        imports = [ ./_vector.nix ];
        logging.vector = {
          enable = true;
          lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
          prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
          tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
          prometheusInputs = [
            "node_exporter"
            "systemd_exporter"
            "process_exporter"
            "smartctl_exporter"
            "otlp_traces.metrics"
          ];
          # Every `systemd --user` start (i.e. every SSH login) warns that it could not
          # arm a PSI trigger on its own cgroup: "Failed to adjust io pressure threshold,
          # ignoring: Device or resource busy". The io controller IS present there
          # (inherited from user.slice) — only the user manager's own pressure watch
          # fails, so nothing observable is lost. Local journal keeps it either way.
          journalDropPatterns = [ "Failed to adjust io pressure threshold" ];
          extraSettings = {
            sources = {
              node_exporter = {
                type = "prometheus_scrape";
                endpoints = [ "http://127.0.0.1:9100/metrics" ];
                scrape_interval_secs = 30;
              };
              systemd_exporter = {
                type = "prometheus_scrape";
                endpoints = [ "http://127.0.0.1:9558/metrics" ];
                scrape_interval_secs = 30;
              };
              process_exporter = {
                type = "prometheus_scrape";
                endpoints = [ "http://127.0.0.1:9256/metrics" ];
                scrape_interval_secs = 30;
              };
              smartctl_exporter = {
                type = "prometheus_scrape";
                endpoints = [ "http://127.0.0.1:9633/metrics" ];
                scrape_interval_secs = 60;
              };
            };
          };
        };

        # NVMe char devices are root:root 0600; smartctl needs them, and no capability
        # bypasses file mode. Block devices are already root:disk.
        services.udev.extraRules = ''
          KERNEL=="nvme[0-9]*", SUBSYSTEM=="nvme", GROUP="disk", MODE="0660"
        '';

        services.prometheus.exporters = {
          node = {
            enable = true;
            enabledCollectors = [
              "systemd"
              "btrfs"
              "hwmon"
              "nvme"
              "rapl"
              "cpufreq"
              "processes"
              "netclass"
              "nfs"
              "mountstats"
            ];
            port = 9100;
            listenAddress = "127.0.0.1";
          };
          process = {
            enable = true;
            port = 9256;
            listenAddress = "127.0.0.1";
            settings.process_names = [
              {
                name = "{{index .Cgroups 0}}";
                cmdline = [ ".+" ];
              }
            ];
          };
          smartctl = {
            enable = true;
            port = 9633;
            listenAddress = "127.0.0.1";
            maxInterval = "60s";
            group = "disk";
          };
          systemd = {
            enable = true;
            port = 9558;
            listenAddress = "127.0.0.1";
            extraFlags = [
              "--systemd.collector.enable-ip-accounting"
            ];
          };
        };

        systemd = {
          services.prometheus-systemd-exporter.serviceConfig.ProtectControlGroups = lib.mkForce false;
          settings.Manager = {
            DefaultCPUAccounting = "yes";
            DefaultMemoryAccounting = "yes";
            DefaultTasksAccounting = "yes";
            DefaultIPAccounting = "yes";
          };
          tmpfiles.rules = [
            "z /sys/class/powercap/*/energy_uj 0444 - - - -"
            "z /sys/class/powercap/*/*/energy_uj 0444 - - - -"
          ];
        };
      };
  };
}
