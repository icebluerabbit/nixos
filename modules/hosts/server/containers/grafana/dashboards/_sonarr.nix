let
  prom = {
    type = "prometheus";
    uid = "Prometheus";
  };
  loki = {
    type = "loki";
    uid = "Loki";
  };
  h = ''host="sonarr"'';
in
{
  folder = "Media";
  title = "Sonarr";
  tags = [
    "media"
    "arr"
    "sonarr"
  ];
  uid = "sonarr";
  editable = true;
  schemaVersion = 38;
  refresh = "30s";
  time = {
    from = "now-24h";
    to = "now";
  };
  panels = [

    # ── Library KPIs ──────────────────────────────────────────────────────
    {
      title = "Series";
      type = "stat";
      id = 1;
      gridPos = {
        h = 4;
        w = 4;
        x = 0;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sonarr_series_total{${h}}";
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
      title = "Episodes";
      type = "stat";
      id = 2;
      gridPos = {
        h = 4;
        w = 4;
        x = 4;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sonarr_episode_total{${h}}";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Missing Episodes";
      type = "stat";
      id = 3;
      gridPos = {
        h = 4;
        w = 4;
        x = 8;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sonarr_episode_missing_total{${h}}";
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
          ];
        };
      };
    }
    {
      title = "Cutoff Unmet";
      type = "stat";
      id = 4;
      gridPos = {
        h = 4;
        w = 4;
        x = 12;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sonarr_episode_cutoff_unmet_total{${h}}";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Library Size";
      type = "stat";
      id = 5;
      gridPos = {
        h = 4;
        w = 4;
        x = 16;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(sonarr_series_filesize_bytes{${h}})";
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "value";
      };
      fieldConfig.defaults.unit = "bytes";
    }
    {
      title = "Health Issues";
      type = "stat";
      id = 6;
      gridPos = {
        h = 4;
        w = 4;
        x = 20;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sonarr_system_health_issues{${h}}";
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
              color = "red";
              value = 1;
            }
          ];
        };
      };
    }

    # ── Episodes over time ────────────────────────────────────────────────
    {
      title = "Episode Counts";
      type = "timeseries";
      id = 7;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "sonarr_episode_total{${h}}";
          legendFormat = "total";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "sonarr_episode_downloaded_total{${h}}";
          legendFormat = "downloaded";
          refId = "B";
        }
        {
          datasource = prom;
          expr = "sonarr_episode_missing_total{${h}}";
          legendFormat = "missing";
          refId = "C";
        }
        {
          datasource = prom;
          expr = "sonarr_episode_cutoff_unmet_total{${h}}";
          legendFormat = "cutoff unmet";
          refId = "D";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        custom = {
          drawStyle = "line";
          fillOpacity = 10;
          lineWidth = 1;
        };
      };
    }
    {
      title = "Seasons: monitored vs downloaded";
      type = "timeseries";
      id = 8;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "sonarr_season_total{${h}}";
          legendFormat = "total";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "sonarr_season_monitored_total{${h}}";
          legendFormat = "monitored";
          refId = "B";
        }
        {
          datasource = prom;
          expr = "sonarr_season_downloaded_total{${h}}";
          legendFormat = "downloaded";
          refId = "C";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        custom = {
          drawStyle = "line";
          fillOpacity = 10;
          lineWidth = 1;
        };
      };
    }

    # ── Activity (VRL) ────────────────────────────────────────────────────
    {
      title = "RSS Reports (found vs grabbed)";
      type = "timeseries";
      id = 9;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "sonarr_rss_reports_found{${h}}";
          legendFormat = "found";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "sonarr_rss_reports_grabbed{${h}}";
          legendFormat = "grabbed";
          refId = "B";
        }
      ];
      fieldConfig.defaults.unit = "short";
    }
    {
      title = "Warnings / hour";
      type = "timeseries";
      id = 10;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(increase(sonarr_warnings_total{${h}}[1h]))";
          legendFormat = "warnings";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    # ── Logs ──────────────────────────────────────────────────────────────
    {
      title = "Grabs / Imports / Warnings";
      type = "logs";
      id = 11;
      gridPos = {
        h = 9;
        w = 24;
        x = 0;
        y = 20;
      };
      targets = [
        {
          datasource = loki;
          expr = ''{host="sonarr", host_type="container"} |~ "Import|Grabbed|RSS Sync Completed|\\[Warn\\]|\\[Error\\]"'';
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
