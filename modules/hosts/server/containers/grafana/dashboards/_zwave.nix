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
  folder = "IoT";
  title = "Z-Wave";
  uid = "zwave";
  tags = [
    "iot"
    "zwave"
    "hass"
  ];
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
      title = "Nodes";
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
          expr = ''count(count by (nodeId) ({__name__=~"zwave_.+"}))'';
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
      title = "Values Tracked";
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
          expr = ''count({__name__=~"zwave_.+"})'';
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
      title = "Switches On";
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
          expr = "sum(zwave_binary_switch_currentvalue)";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    # ── Device value table ────────────────────────────────────────────────
    {
      title = "Device Values";
      type = "table";
      id = 4;
      gridPos = {
        h = 10;
        w = 24;
        x = 0;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = ''{__name__=~"zwave_.+"}'';
          format = "table";
          instant = true;
          refId = "A";
        }
      ];
      transformations = [
        {
          id = "organize";
          options.excludeByName = {
            Time = true;
            __name__ = false;
            job = true;
            instance = true;
            host = true;
            host_type = true;
            commandClass = false;
            endpoint = true;
          };
        }
      ];
      fieldConfig.defaults.custom.filterable = true;
    }

    # ── Binary switch states over time ────────────────────────────────────
    {
      title = "Binary Switch State";
      type = "timeseries";
      id = 5;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 14;
      };
      targets = [
        {
          datasource = prom;
          expr = "zwave_binary_switch_currentvalue";
          legendFormat = "{{name}} ({{location}})";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        min = 0;
        max = 1;
        custom = {
          drawStyle = "line";
          lineInterpolation = "stepAfter";
          fillOpacity = 20;
          lineWidth = 2;
        };
      };
    }

    # ── Logs ──────────────────────────────────────────────────────────────
    {
      title = "Z-Wave JS UI Logs";
      type = "logs";
      id = 6;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 22;
      };
      targets = [
        {
          datasource = loki;
          expr = ''{host="zwave", host_type="container"}'';
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
