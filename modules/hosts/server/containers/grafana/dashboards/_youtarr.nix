{
  folder = "Media";
  title = "Youtarr";
  tags = [
    "media"
    "download"
    "youtube"
  ];
  uid = "youtarr";
  editable = true;
  schemaVersion = 38;
  refresh = "30s";
  time = {
    from = "now-7d";
    to = "now";
  };
  panels = [

    # ── KPIs ─────────────────────────────────────────────────────────────
    {
      title = "Downloads Completed";
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
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum(increase(youtarr_downloads_completed_total[$__range]))";
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
      title = "Current Speed";
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
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "youtarr_download_speed_bytes_per_second";
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "value";
        graphMode = "area";
      };
      fieldConfig.defaults.unit = "binBps";
    }
    {
      title = "Current Progress";
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
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "youtarr_download_percent";
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "value";
        graphMode = "area";
      };
      fieldConfig.defaults = {
        unit = "percent";
        min = 0;
        max = 100;
      };
    }

    # ── Download Speed ────────────────────────────────────────────────────
    {
      title = "Download Speed";
      type = "timeseries";
      id = 4;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 4;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "youtarr_download_speed_bytes_per_second";
          legendFormat = "speed";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "binBps";
    }

    # ── Completion Breakdown ──────────────────────────────────────────────
    {
      title = "Downloads by Category";
      type = "piechart";
      id = 5;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum by (category) (increase(youtarr_downloads_completed_total[$__range]))";
          legendFormat = "{{category}}";
          refId = "A";
        }
      ];
      options.displayLabels = [
        "name"
        "percent"
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Downloads Completed per Day";
      type = "timeseries";
      id = 6;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 12;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum by (category) (increase(youtarr_downloads_completed_total[1d]))";
          legendFormat = "{{category}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    # ── Recent Completions Log ────────────────────────────────────────────
    {
      title = "Recent Downloads";
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
          datasource = {
            type = "loki";
            uid = "Loki";
          };
          expr = ''{host="youtarr", host_type="container"} |= "Marked video as completed in tracking"'';
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
