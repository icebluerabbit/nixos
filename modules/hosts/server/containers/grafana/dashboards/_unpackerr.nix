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
  title = "Unpackerr";
  tags = [
    "media"
    "arr"
    "unpackerr"
  ];
  uid = "unpackerr";
  editable = true;
  schemaVersion = 38;
  refresh = "30s";
  time = {
    from = "now-6h";
    to = "now";
  };
  panels = [

    # ── KPIs ──────────────────────────────────────────────────────────────
    {
      title = "Uptime";
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
          expr = "unpackerr_uptime_seconds_total";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      fieldConfig.defaults.unit = "s";
    }
    {
      title = "Extracted (now)";
      type = "stat";
      id = 2;
      gridPos = {
        h = 4;
        w = 5;
        x = 4;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(unpackerr_queue_extracted)";
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
      title = "Imported (session)";
      type = "stat";
      id = 3;
      gridPos = {
        h = 4;
        w = 5;
        x = 9;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(increase(unpackerr_imported_total[$__range]))";
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
    {
      title = "Failed (now)";
      type = "stat";
      id = 4;
      gridPos = {
        h = 4;
        w = 5;
        x = 14;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(unpackerr_queue_failed)";
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
    {
      title = "Waiting (now)";
      type = "stat";
      id = 5;
      gridPos = {
        h = 4;
        w = 5;
        x = 19;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(unpackerr_queue_waiting)";
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "value";
      };
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    # ── Queue pipeline over time ──────────────────────────────────────────
    {
      title = "Extraction Pipeline";
      type = "timeseries";
      id = 6;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(unpackerr_queue_waiting)";
          legendFormat = "waiting";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "sum(unpackerr_queue_extracting)";
          legendFormat = "extracting";
          refId = "B";
        }
        {
          datasource = prom;
          expr = "sum(unpackerr_queue_extracted)";
          legendFormat = "extracted";
          refId = "C";
        }
        {
          datasource = prom;
          expr = "sum(unpackerr_queue_imported)";
          legendFormat = "imported";
          refId = "D";
        }
        {
          datasource = prom;
          expr = "sum(unpackerr_queue_failed)";
          legendFormat = "failed";
          refId = "E";
        }
        {
          datasource = prom;
          expr = "sum(unpackerr_queue_deleted)";
          legendFormat = "deleted";
          refId = "F";
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

    # ── Per-app queue + fetch latency ─────────────────────────────────────
    {
      title = "Queue Size by App";
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
          expr = "unpackerr_app_queue_size";
          legendFormat = "{{app}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        custom = {
          drawStyle = "bars";
          fillOpacity = 40;
        };
      };
    }
    {
      title = "Queue Fetch Latency by App";
      type = "timeseries";
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
          expr = "unpackerr_app_queue_fetch_time_seconds";
          legendFormat = "{{app}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "s";
    }

    # ── Activity log ──────────────────────────────────────────────────────
    {
      title = "Extract / Import Activity";
      type = "logs";
      id = 9;
      gridPos = {
        h = 9;
        w = 24;
        x = 0;
        y = 20;
      };
      targets = [
        {
          datasource = loki;
          expr = ''{host="unpackerr", host_type="container"} |~ "Extracted|Imported|no extractable|failed"'';
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
