let
  prom = {
    type = "prometheus";
    uid = "Prometheus";
  };
  dev = ''{host=~"$host"}'';
in
{
  folder = "Infrastructure";
  title = "SMART";
  uid = "smart";
  tags = [
    "infra"
    "smart"
    "storage"
    "disk"
  ];
  editable = true;
  schemaVersion = 38;
  refresh = "1m";
  time = {
    from = "now-7d";
    to = "now";
  };
  templating.list = [
    {
      name = "host";
      type = "query";
      datasource = prom;
      query = "label_values(smartctl_device_percentage_used, host)";
      refresh = 1;
      includeAll = true;
      multi = true;
      current = { };
    }
  ];
  panels = [

    # ── KPIs ──────────────────────────────────────────────────────────────
    {
      title = "Drives Monitored";
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
          expr = "count(smartctl_device_percentage_used${dev})";
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
      title = "Critical Warnings";
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
          expr = "sum(smartctl_device_critical_warning${dev})";
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
      title = "Worst Wearout";
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
          expr = "max(smartctl_device_percentage_used${dev})";
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults = {
        unit = "percent";
        max = 100;
        thresholds = {
          mode = "absolute";
          steps = [
            {
              color = "green";
              value = null;
            }
            {
              color = "yellow";
              value = 60;
            }
            {
              color = "red";
              value = 80;
            }
          ];
        };
      };
    }
    {
      title = "Hottest Drive";
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
          expr = "max(smartctl_device_temperature${dev})";
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults = {
        unit = "celsius";
        thresholds = {
          mode = "absolute";
          steps = [
            {
              color = "green";
              value = null;
            }
            {
              color = "yellow";
              value = 60;
            }
            {
              color = "red";
              value = 70;
            }
          ];
        };
      };
    }

    # ── Health ────────────────────────────────────────────────────────────
    {
      title = "Wearout by Drive";
      type = "timeseries";
      id = 10;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "smartctl_device_percentage_used${dev}";
          legendFormat = "{{host}} {{device}} {{model_name}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "percent";
        max = 100;
      };
    }
    {
      title = "Temperature by Drive";
      type = "timeseries";
      id = 11;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 4;
      };
      targets = [
        {
          datasource = prom;
          expr = "smartctl_device_temperature${dev}";
          legendFormat = "{{host}} {{device}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "celsius";
    }
    {
      title = "Available Spare vs Threshold";
      type = "timeseries";
      id = 20;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "smartctl_device_available_spare${dev}";
          legendFormat = "spare {{host}} {{device}}";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "smartctl_device_available_spare_threshold${dev}";
          legendFormat = "threshold {{host}} {{device}}";
          refId = "B";
        }
      ];
      fieldConfig.defaults.unit = "percent";
    }
    {
      title = "Media Errors & Error Log Entries";
      type = "timeseries";
      id = 21;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "smartctl_device_media_errors${dev}";
          legendFormat = "media errors {{host}} {{device}}";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "smartctl_device_num_err_log_entries${dev}";
          legendFormat = "err log {{host}} {{device}}";
          refId = "B";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    # ── Lifetime ──────────────────────────────────────────────────────────
    {
      title = "Power-On Time";
      type = "timeseries";
      id = 30;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 20;
      };
      targets = [
        {
          datasource = prom;
          expr = "smartctl_device_power_on_seconds${dev}";
          legendFormat = "{{host}} {{device}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "s";
    }
    {
      title = "Data Written";
      type = "timeseries";
      id = 31;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 20;
      };
      targets = [
        {
          datasource = prom;
          expr = "smartctl_device_bytes_written${dev}";
          legendFormat = "{{host}} {{device}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "bytes";
    }
  ];
}
