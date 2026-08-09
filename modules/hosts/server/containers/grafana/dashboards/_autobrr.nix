let
  prom = {
    type = "prometheus";
    uid = "Prometheus";
  };
  loki = {
    type = "loki";
    uid = "Loki";
  };
  h = ''host="autobrr"'';
in
{
  folder = "Media";
  title = "autobrr";
  tags = [
    "media"
    "download"
    "autobrr"
  ];
  uid = "autobrr";
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
      title = "Push Approved";
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
          expr = "delta(autobrr_push_approved_total{${h}}[$__range])";
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
      title = "Push Rejected";
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
          expr = "delta(autobrr_push_rejected_total{${h}}[$__range])";
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
    {
      title = "Push Errors";
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
          expr = "delta(autobrr_push_errors_total{${h}}[$__range])";
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
      title = "IRC Connected";
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
          expr = "autobrr_irc_connected{${h}}";
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
              color = "red";
              value = null;
            }
            {
              color = "green";
              value = 1;
            }
          ];
        };
      };
    }

    # ── Push activity over time ───────────────────────────────────────────
    {
      title = "Push Rate";
      type = "timeseries";
      id = 5;
      gridPos = {
        h = 8;
        w = 16;
        x = 0;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "deriv(autobrr_push_approved_total{${h}}[10m])";
          legendFormat = "approved";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "deriv(autobrr_push_rejected_total{${h}}[10m])";
          legendFormat = "rejected";
          refId = "B";
        }
        {
          datasource = prom;
          expr = "deriv(autobrr_push_errors_total{${h}}[10m])";
          legendFormat = "errors";
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

    # ── IRC status ────────────────────────────────────────────────────────
    {
      title = "IRC Status";
      type = "timeseries";
      id = 6;
      gridPos = {
        h = 8;
        w = 8;
        x = 16;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "autobrr_irc_connected{${h}}";
          legendFormat = "connected";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "autobrr_irc_healthy{${h}}";
          legendFormat = "healthy";
          refId = "B";
        }
        {
          datasource = prom;
          expr = "autobrr_up{${h}}";
          legendFormat = "up";
          refId = "C";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
        min = 0;
      };
    }

    # ── Logs ─────────────────────────────────────────────────────────────
    {
      title = "Recent Activity";
      type = "logs";
      id = 7;
      gridPos = {
        h = 9;
        w = 24;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = loki;
          expr = ''{host="autobrr", host_type="container"}'';
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
