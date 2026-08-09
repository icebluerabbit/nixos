# LazyLibrarian has no exportarr and exposes no /metrics — every series here is
# derived from its journal by the Vector sidecar (see the container's logging
# .extraSettings). That caps what's showable: no queue depth, no library counts,
# no disk stats. Counters reset when Vector restarts, hence increase() throughout.
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
  title = "LazyLibrarian";
  tags = [
    "media"
    "books"
    "lazylibrarian"
  ];
  uid = "lazylibrarian";
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
      title = "Imports (24h)";
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
          expr = "sum(increase(lazylibrarian_imports_total[24h]))";
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
      title = "Books Found (last search)";
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
          expr = "lazylibrarian_search_books_found";
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
      title = "Downloads Processed (last pass)";
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
          expr = "lazylibrarian_downloads_processed";
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
      title = "Warnings / Errors (24h)";
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
          expr = "sum(increase(lazylibrarian_warnings_total[24h]))";
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
            color = "yellow";
            value = 10;
          }
          {
            color = "red";
            value = 50;
          }
        ];
      };
    }

    # ── Activity ─────────────────────────────────────────────────────────
    {
      title = "Imports";
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
          expr = "sum(increase(lazylibrarian_imports_total[1h]))";
          legendFormat = "imports/h";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Search Yield";
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
          expr = "lazylibrarian_search_books_found";
          legendFormat = "books found";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "lazylibrarian_downloads_processed";
          legendFormat = "downloads processed";
          refId = "B";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Warnings / Errors";
      type = "timeseries";
      id = 7;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum(increase(lazylibrarian_warnings_total[1h]))";
          legendFormat = "warnings+errors/h";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
        custom.fillOpacity = 20;
      };
    }

    # ── Logs ─────────────────────────────────────────────────────────────
    {
      title = "Warnings & Errors";
      type = "logs";
      id = 8;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 20;
      };
      targets = [
        {
          datasource = loki;
          # LL's python logging emits "WARNING:"/"ERROR:", not the *arrs' [Warn]/[Error].
          # Config[BOK_LOGIN|BOK_USER] are an upstream bug (api.py reads keys configdefs
          # never defines) — 4 per configuratarr sync, pure noise. Same exclusion as the
          # warn metric in the container's Vector config.
          expr = ''{host="lazylibrarian", host_type="container"} |~ "(WARNING|ERROR):" != "Config[BOK_"'';
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
      id = 9;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 28;
      };
      targets = [
        {
          datasource = loki;
          expr = ''{host="lazylibrarian", host_type="container"}'';
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
