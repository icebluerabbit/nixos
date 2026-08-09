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
  title = "cross-seed";
  tags = [
    "media"
    "download"
    "cross-seed"
  ];
  uid = "cross-seed";
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
      title = "Injections";
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
          expr = "sum(increase(cross_seed_injections_total[$__range]))";
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
      title = "Injected (last run)";
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
          expr = "cross_seed_injected_recent";
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
      title = "Warnings";
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
          expr = "sum(increase(cross_seed_warnings_total[$__range]))";
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
      title = "Errors";
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
          expr = "sum(increase(cross_seed_errors_total[$__range]))";
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

    # ── Injections ───────────────────────────────────────────────────────
    {
      title = "Injections by Result";
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
          expr = "sum by (result) (increase(cross_seed_injections_total[1h]))";
          legendFormat = "{{result}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Candidates vs Injected (last run)";
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
          expr = "cross_seed_candidates_recent";
          legendFormat = "candidates";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "cross_seed_injected_recent";
          legendFormat = "injected";
          refId = "B";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    # ── Errors / Warnings ────────────────────────────────────────────────
    {
      title = "Errors & Warnings Rate";
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
          expr = "sum(rate(cross_seed_errors_total[5m]))";
          legendFormat = "errors";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "sum(rate(cross_seed_warnings_total[5m]))";
          legendFormat = "warnings";
          refId = "B";
        }
      ];
      fieldConfig.defaults.unit = "short";
    }

    # ── Logs ─────────────────────────────────────────────────────────────
    {
      title = "Recent Activity";
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
          expr = ''{host="cross-seed", host_type="container"} |~ "Injected|Found|error:|warn:"'';
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
