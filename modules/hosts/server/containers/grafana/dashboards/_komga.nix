# Komga ships spring-boot-starter-actuator but NOT micrometer-registry-prometheus
# — its OpenAPI exposes only /actuator/info, so there is no /actuator/prometheus
# to scrape. Every series here is derived from Komga's journal by the Vector
# sidecar (see the container's logging.extraSettings). That caps what's showable:
# no library/series/book counts, no scan durations, no disk stats. Counters reset
# when Vector restarts, hence increase() throughout.
#
# The two filters are deliberately broad — these patterns were written before any
# Komga log line had been observed. Tighten them against real output once the
# container has been running (journalctl -M komga).
let
  prom = {
    type = "prometheus";
    uid = "Prometheus";
  };
  loki = {
    type = "loki";
    uid = "Loki";
  };
in
{
  folder = "Media";
  title = "Komga";
  tags = [
    "media"
    "comics"
    "komga"
  ];
  uid = "komga";
  editable = true;
  schemaVersion = 38;
  refresh = "30s";
  time = {
    from = "now-24h";
    to = "now";
  };
  panels = [

    # ── KPIs ─────────────────────────────────────────────────────────────
    {
      title = "Scans (24h)";
      type = "stat";
      id = 1;
      gridPos = {
        h = 4;
        w = 8;
        x = 0;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(increase(komga_scans_total[24h]))";
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Warnings & Errors (24h)";
      type = "stat";
      id = 2;
      gridPos = {
        h = 4;
        w = 8;
        x = 8;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(increase(komga_warnings_total[24h]))";
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
        thresholds = {
          mode = "absolute";
          steps = [
            {
              color = "green";
              value = null;
            }
            {
              color = "yellow";
              value = 1;
            }
            {
              color = "red";
              value = 10;
            }
          ];
        };
      };
    }
    {
      title = "Log Volume (24h)";
      type = "stat";
      id = 3;
      gridPos = {
        h = 4;
        w = 8;
        x = 16;
        y = 0;
      };
      targets = [
        {
          datasource = loki;
          expr = ''sum(count_over_time({host="komga", host_type="container"}[24h]))'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    # ── Rates ────────────────────────────────────────────────────────────
    {
      title = "Scan Rate";
      type = "timeseries";
      id = 4;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(rate(komga_scans_total[15m]))";
          legendFormat = "scans";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "short";
    }
    {
      title = "Warning & Error Rate";
      type = "timeseries";
      id = 5;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(rate(komga_warnings_total[15m]))";
          legendFormat = "warn+error";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "short";
    }

    # ── Logs ─────────────────────────────────────────────────────────────
    {
      title = "Warnings & Errors";
      type = "logs";
      id = 6;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = loki;
          # Spring Boot's default pattern prints the bare level, not [Warn].
          expr = ''{host="komga", host_type="container"} |~ "\\bWARN\\b|\\bERROR\\b"'';
          refId = "A";
        }
      ];
      options = {
        showTime = true;
        sortOrder = "Descending";
        wrapLogMessage = true;
      };
    }
    {
      title = "Recent Activity";
      type = "logs";
      id = 7;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 20;
      };
      targets = [
        {
          datasource = loki;
          expr = ''{host="komga", host_type="container"}'';
          refId = "A";
        }
      ];
      options = {
        showTime = true;
        sortOrder = "Descending";
        wrapLogMessage = true;
      };
    }

  ];
}
