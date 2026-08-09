{
  folder = "Network";
  title = "Network Logs";
  tags = [
    "network"
    "logs"
  ];
  uid = "network-logs";
  editable = true;
  schemaVersion = 38;
  refresh = "5s";
  time = {
    from = "now-1h";
    to = "now";
  };
  templating.list = [
    {
      name = "host";
      type = "query";
      datasource = {
        type = "loki";
        uid = "Loki";
      };
      query = {
        refId = "LokiVariableQuery";
        type = 1;
        label = "host";
        stream = "{host_type=\"network\"}";
      };
      refresh = 1;
      includeAll = true;
      multi = true;
      current = { };
    }
  ];
  panels = [
    {
      title = "Log Rate";
      type = "stat";
      id = 4;
      gridPos = {
        h = 4;
        w = 8;
        x = 0;
        y = 0;
      };
      targets = [
        {
          datasource = {
            type = "loki";
            uid = "Loki";
          };
          expr = "sum(rate({host_type=\"network\",host=~\"$host\"}[5m]))";
          refId = "A";
        }
      ];
      options = {
        colorMode = "value";
        graphMode = "area";
        justifyMode = "auto";
        reduceOptions = {
          calcs = [ "lastNotNull" ];
          fields = "";
          values = false;
        };
      };
    }
    {
      title = "Total Logs Ingested";
      type = "stat";
      id = 5;
      gridPos = {
        h = 4;
        w = 8;
        x = 8;
        y = 0;
      };
      targets = [
        {
          datasource = {
            type = "loki";
            uid = "Loki";
          };
          expr = "sum(count_over_time({host_type=\"network\",host=~\"$host\"}[$__range]))";
          refId = "A";
        }
      ];
      options = {
        colorMode = "value";
        graphMode = "area";
        justifyMode = "auto";
        reduceOptions = {
          calcs = [ "lastNotNull" ];
          fields = "";
          values = false;
        };
      };
    }
    {
      title = "Total Errors";
      type = "stat";
      id = 6;
      gridPos = {
        h = 4;
        w = 8;
        x = 16;
        y = 0;
      };
      targets = [
        {
          datasource = {
            type = "loki";
            uid = "Loki";
          };
          expr = "sum(count_over_time({host_type=\"network\",host=~\"$host\"} | detected_level=\"error\"[$__range]))";
          refId = "A";
        }
      ];
      options = {
        colorMode = "value";
        graphMode = "area";
        justifyMode = "auto";
        reduceOptions = {
          calcs = [ "lastNotNull" ];
          fields = "";
          values = false;
        };
      };
      fieldConfig = {
        defaults = {
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
      };
    }
    {
      title = "Log Volume over Time";
      type = "timeseries";
      id = 1;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 4;
      };
      targets = [
        {
          datasource = {
            type = "loki";
            uid = "Loki";
          };
          expr = "sum by (host) (count_over_time({host_type=\"network\",host=~\"$host\"}[$__interval]))";
          legendFormat = "{{host}}";
          refId = "A";
        }
      ];
    }
    {
      title = "Error Volume over Time";
      type = "timeseries";
      id = 3;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = {
            type = "loki";
            uid = "Loki";
          };
          expr = "sum by (host) (count_over_time({host_type=\"network\",host=~\"$host\"} | detected_level=\"error\"[$__interval]))";
          legendFormat = "{{host}}";
          refId = "A";
        }
      ];
      fieldConfig = {
        defaults = {
          custom = {
            drawStyle = "line";
            lineInterpolation = "smooth";
          };
        };
      };
    }
    {
      title = "Logs";
      type = "logs";
      id = 2;
      gridPos = {
        h = 16;
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
          expr = "{host_type=\"network\",host=~\"$host\"} | json | line_format \"{{.message}}\"";
          refId = "A";
        }
      ];
      options = {
        showLabels = true;
        wrapLogMessage = true;
        enableLogDetails = true;
        showTime = true;
      };
    }
    {
      title = "Error Logs";
      type = "logs";
      id = 7;
      gridPos = {
        h = 16;
        w = 24;
        x = 0;
        y = 36;
      };
      targets = [
        {
          datasource = {
            type = "loki";
            uid = "Loki";
          };
          expr = "{host_type=\"network\",host=~\"$host\"} | detected_level=\"error\" | json | line_format \"{{.message}}\"";
          refId = "A";
        }
      ];
      options = {
        showLabels = true;
        wrapLogMessage = true;
        enableLogDetails = true;
        showTime = true;
      };
    }
  ];
}
