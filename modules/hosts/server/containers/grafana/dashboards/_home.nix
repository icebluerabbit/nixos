{
  title = "Home";
  tags = [
    "overview"
  ];
  uid = "home";
  editable = true;
  schemaVersion = 38;
  refresh = "10s";
  time = {
    from = "now-1h";
    to = "now";
  };
  panels = [
    {
      title = "Active Containers";
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
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum(node_systemd_unit_state{state=\"active\",name=~\"container@.*\"})";
          refId = "A";
        }
      ];
      options = {
        colorMode = "value";
        graphMode = "none";
        reduceOptions = {
          calcs = [ "lastNotNull" ];
          fields = "";
          values = false;
        };
      };
    }
    {
      title = "Failed Systemd Units";
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
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum(node_systemd_unit_state{state=\"failed\"})";
          refId = "A";
        }
      ];
      options = {
        colorMode = "value";
        graphMode = "none";
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
      title = "Log Ingestion Rate";
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
          datasource = {
            type = "loki";
            uid = "Loki";
          };
          expr = "sum(rate({host_type=\"host\"}[5m]))";
          refId = "A";
        }
      ];
      options = {
        colorMode = "value";
        graphMode = "area";
        reduceOptions = {
          calcs = [ "lastNotNull" ];
          fields = "";
          values = false;
        };
      };
    }
    {
      title = "System Error Logs (Last 1h)";
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
          datasource = {
            type = "loki";
            uid = "Loki";
          };
          expr = "sum(count_over_time({host_type=\"host\"} | detected_level=\"error\"[1h]))";
          refId = "A";
        }
      ];
      options = {
        colorMode = "value";
        graphMode = "area";
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
                color = "yellow";
                value = 10;
              }
              {
                color = "red";
                value = 100;
              }
            ];
          };
        };
      };
    }
    {
      title = "Container Services Status Timeline";
      type = "state-timeline";
      id = 5;
      gridPos = {
        h = 40;
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
          expr = "node_systemd_unit_state{name=~\"container@.*\",state=\"active\"}";
          legendFormat = "{{name}}";
          refId = "A";
        }
      ];
      options = {
        showValue = "never";
        align = "left";
        mergeValues = true;
        rowHeight = 0.5;
      };
      fieldConfig = {
        defaults = {
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
      };
    }
    {
      title = "Dashboard Directory";
      type = "dashlist";
      id = 6;
      gridPos = {
        h = 10;
        w = 24;
        x = 0;
        y = 44;
      };
      options = {
        showStarred = false;
        showRecentlyViewed = false;
        showSearch = true;
        showHeadings = true;
        maxItems = 20;
        query = "";
        tags = [ ];
      };
    }
  ];
}
