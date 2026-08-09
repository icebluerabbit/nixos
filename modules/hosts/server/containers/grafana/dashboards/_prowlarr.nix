let
  prom = {
    type = "prometheus";
    uid = "Prometheus";
  };
  loki = {
    type = "loki";
    uid = "Loki";
  };
  h = ''host="prowlarr"'';
in
{
  folder = "Media";
  title = "Prowlarr";
  tags = [
    "media"
    "arr"
    "prowlarr"
    "indexer"
  ];
  uid = "prowlarr";
  editable = true;
  schemaVersion = 38;
  refresh = "30s";
  time = {
    from = "now-24h";
    to = "now";
  };
  panels = [

    # ── KPIs ──────────────────────────────────────────────────────────────
    {
      title = "Indexers";
      type = "stat";
      id = 1;
      gridPos = {
        h = 4;
        w = 5;
        x = 0;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "prowlarr_indexer_enabled_total{${h}}";
          legendFormat = "enabled";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "prowlarr_indexer_total{${h}}";
          legendFormat = "total";
          refId = "B";
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
      title = "Queries (session)";
      type = "stat";
      id = 2;
      gridPos = {
        h = 4;
        w = 5;
        x = 5;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(increase(prowlarr_indexer_queries_total{${h}}[$__range]))";
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
      title = "Grabs (session)";
      type = "stat";
      id = 3;
      gridPos = {
        h = 4;
        w = 5;
        x = 10;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(increase(prowlarr_indexer_grabs_total{${h}}[$__range]))";
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
      title = "Failed Queries (session)";
      type = "stat";
      id = 4;
      gridPos = {
        h = 4;
        w = 5;
        x = 15;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(increase(prowlarr_indexer_failed_queries_total{${h}}[$__range]))";
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
      title = "Health Issues";
      type = "stat";
      id = 5;
      gridPos = {
        h = 4;
        w = 4;
        x = 20;
        y = 0;
      };
      targets = [
        {
          datasource = prom;
          expr = "prowlarr_system_health_issues{${h}}";
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

    # ── Query / grab activity by indexer ──────────────────────────────────
    {
      title = "Queries / min by Indexer";
      type = "timeseries";
      id = 6;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum by (indexer) (rate(prowlarr_indexer_queries_total{${h}}[5m]) * 60)";
          legendFormat = "{{indexer}}";
          refId = "A";
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
      title = "Grabs / min by Indexer";
      type = "timeseries";
      id = 7;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum by (indexer) (rate(prowlarr_indexer_grabs_total{${h}}[5m]) * 60)";
          legendFormat = "{{indexer}}";
          refId = "A";
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

    # ── Response time + failures ──────────────────────────────────────────
    {
      title = "Indexer Response Time";
      type = "timeseries";
      id = 8;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "prowlarr_indexer_response_time_seconds{${h}}";
          legendFormat = "{{indexer}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "s";
    }
    {
      title = "Failed Queries / min by Indexer";
      type = "timeseries";
      id = 9;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum by (indexer) (rate(prowlarr_indexer_failed_queries_total{${h}}[5m]) * 60)";
          legendFormat = "{{indexer}}";
          refId = "A";
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

    # ── Logs ──────────────────────────────────────────────────────────────
    {
      title = "Prowlarr Warnings / Errors";
      type = "logs";
      id = 10;
      gridPos = {
        h = 9;
        w = 24;
        x = 0;
        y = 20;
      };
      targets = [
        {
          datasource = loki;
          expr = ''{host="prowlarr", host_type="container"} |~ "\\[Warn\\]|\\[Error\\]|Grabbed|Search"'';
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
