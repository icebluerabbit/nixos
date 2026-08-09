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
  title = "Bazarr";
  tags = [
    "media"
    "subtitles"
    "bazarr"
  ];
  uid = "bazarr";
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
      title = "Wanted Episodes";
      type = "stat";
      id = 1;
      gridPos = {
        h = 4;
        w = 6;
        x = 0;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "bazarr_wanted_episodes";
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
      title = "Wanted Movies";
      type = "stat";
      id = 2;
      gridPos = {
        h = 4;
        w = 6;
        x = 6;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "bazarr_wanted_movies";
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
      title = "Providers Throttled";
      type = "stat";
      id = 3;
      gridPos = {
        h = 4;
        w = 6;
        x = 12;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "bazarr_providers_throttled";
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
        thresholds.steps = [
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
    }
    {
      title = "Health Issues";
      type = "stat";
      id = 4;
      gridPos = {
        h = 4;
        w = 6;
        x = 18;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "bazarr_health_issues";
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
        thresholds.steps = [
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
    }

    # ── Wanted over time + downloads ─────────────────────────────────────
    {
      title = "Wanted Subtitles";
      type = "timeseries";
      id = 5;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "bazarr_wanted_episodes";
          legendFormat = "episodes";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "bazarr_wanted_movies";
          legendFormat = "movies";
          refId = "B";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Downloads (history window)";
      type = "timeseries";
      id = 6;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "bazarr_downloads_series_window";
          legendFormat = "series";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "bazarr_downloads_movies_window";
          legendFormat = "movies";
          refId = "B";
        }
        {
          datasource = prom;
          expr = "bazarr_downloads_total_window";
          legendFormat = "total";
          refId = "C";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    # ── Connectivity ─────────────────────────────────────────────────────
    {
      title = "Sonarr / Radarr Connection";
      type = "timeseries";
      id = 7;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "bazarr_sonarr_up";
          legendFormat = "sonarr";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "bazarr_radarr_up";
          legendFormat = "radarr";
          refId = "B";
        }
        {
          datasource = prom;
          expr = "bazarr_up";
          legendFormat = "bazarr";
          refId = "C";
        }
      ];
      fieldConfig.defaults = {
        unit = "bool";
        decimals = 0;
        min = 0;
        max = 1;
      };
    }
    {
      title = "Downloaded (total, window)";
      type = "stat";
      id = 8;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "bazarr_downloads_total_window";
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "value";
        graphMode = "area";
      };
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    # ── Logs ─────────────────────────────────────────────────────────────
    {
      title = "Recent Activity";
      type = "logs";
      id = 9;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 20;
      };
      targets = [
        {
          datasource = loki;
          expr = ''{host="bazarr", host_type="container"}'';
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
