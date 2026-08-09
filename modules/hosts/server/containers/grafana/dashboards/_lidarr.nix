let
  prom = {
    type = "prometheus";
    uid = "Prometheus";
  };
  loki = {
    type = "loki";
    uid = "Loki";
  };
  h = ''host="lidarr"'';
in
{
  folder = "Media";
  title = "Lidarr";
  tags = [
    "media"
    "arr"
    "lidarr"
  ];
  uid = "lidarr";
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
      title = "Artists";
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
          expr = "lidarr_artists_total{${h}}";
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
      title = "Albums";
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
          expr = "lidarr_albums_total{${h}}";
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
      title = "Missing Albums";
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
          expr = "lidarr_albums_missing_total{${h}}";
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
      title = "Songs";
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
          expr = "lidarr_songs_total{${h}}";
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
          expr = "sum(lidarr_artists_filesize_bytes{${h}})";
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
          expr = "lidarr_system_health_issues{${h}}";
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

    # ── Library over time ─────────────────────────────────────────────────
    {
      title = "Album Counts";
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
          expr = "lidarr_albums_total{${h}}";
          legendFormat = "total";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "lidarr_albums_monitored_total{${h}}";
          legendFormat = "monitored";
          refId = "B";
        }
        {
          datasource = prom;
          expr = "lidarr_albums_missing_total{${h}}";
          legendFormat = "missing";
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
    {
      title = "Songs: monitored vs downloaded";
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
          expr = "lidarr_songs_total{${h}}";
          legendFormat = "total";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "lidarr_songs_monitored_total{${h}}";
          legendFormat = "monitored";
          refId = "B";
        }
        {
          datasource = prom;
          expr = "lidarr_songs_downloaded_total{${h}}";
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

    # ── Queue ─────────────────────────────────────────────────────────────
    {
      title = "Queue by status";
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
          expr = "sum by (status) (lidarr_queue_total{${h}})";
          legendFormat = "{{status}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    # ── Activity (VRL) ────────────────────────────────────────────────────
    {
      title = "RSS Reports (found vs grabbed)";
      type = "timeseries";
      id = 10;
      gridPos = {
        h = 8;
        w = 6;
        x = 12;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "lidarr_rss_reports_found{${h}}";
          legendFormat = "found";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "lidarr_rss_reports_grabbed{${h}}";
          legendFormat = "grabbed";
          refId = "B";
        }
      ];
      fieldConfig.defaults.unit = "short";
    }
    {
      title = "Warnings / hour";
      type = "timeseries";
      id = 11;
      gridPos = {
        h = 8;
        w = 6;
        x = 18;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(increase(lidarr_warnings_total{${h}}[1h]))";
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
      id = 12;
      gridPos = {
        h = 9;
        w = 24;
        x = 0;
        y = 20;
      };
      targets = [
        {
          datasource = loki;
          expr = ''{host="lidarr", host_type="container"} |~ "Import|Grabbed|RSS Sync Completed|\\[Warn\\]|\\[Error\\]"'';
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
