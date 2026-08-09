{
  folder = "Infrastructure";
  title = "Host Metrics";
  tags = [
    "infra"
    "metrics"
    "host"
  ];
  uid = "host-metrics";
  editable = true;
  schemaVersion = 38;
  refresh = "10s";
  time = {
    from = "now-1h";
    to = "now";
  };
  templating.list = [
    {
      name = "host";
      type = "query";
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      query = "label_values(node_cpu_seconds_total{host_type=\"host\"}, host)";
      refresh = 1;
      includeAll = false;
      multi = false;
      current = { };
    }
  ];
  panels = [
    {
      title = "CPU Usage";
      type = "timeseries";
      id = 1;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 0;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum by (mode) (rate(node_cpu_seconds_total{host=\"$host\"}[5m])) * 100 / scalar(count(count by (cpu) (node_cpu_seconds_total{host=\"$host\"})))";
          legendFormat = "{{mode}}";
          refId = "A";
        }
      ];
      fieldConfig = {
        defaults = {
          unit = "percent";
        };
      };
    }
    {
      title = "Memory Usage";
      type = "timeseries";
      id = 2;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 0;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "node_memory_MemTotal_bytes{host=\"$host\"} - node_memory_MemFree_bytes{host=\"$host\"} - node_memory_Buffers_bytes{host=\"$host\"} - node_memory_Cached_bytes{host=\"$host\"}";
          legendFormat = "Used";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "node_memory_Cached_bytes{host=\"$host\"} + node_memory_Buffers_bytes{host=\"$host\"}";
          legendFormat = "Cached/Buffers";
          refId = "B";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "node_memory_MemFree_bytes{host=\"$host\"}";
          legendFormat = "Free";
          refId = "C";
        }
      ];
      fieldConfig = {
        defaults = {
          unit = "bytes";
          custom = {
            stacking = {
              mode = "normal";
              group = "A";
            };
          };
        };
      };
    }
    {
      title = "Temperatures";
      type = "timeseries";
      id = 3;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 8;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "node_hwmon_temp_celsius{host=\"$host\"}";
          legendFormat = "{{sensor}} ({{chip}})";
          refId = "A";
        }
      ];
      fieldConfig = {
        defaults = {
          unit = "celsius";
        };
      };
    }
    {
      title = "CPU Power Draw";
      type = "timeseries";
      id = 4;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 8;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "rate(node_rapl_package_joules_total{host=\"$host\"}[5m])";
          legendFormat = "CPU Package";
          refId = "A";
        }
      ];
      fieldConfig = {
        defaults = {
          unit = "watt";
        };
      };
    }
    {
      title = "Disk Throughput";
      type = "timeseries";
      id = 5;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 16;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "rate(node_disk_read_bytes_total{host=\"$host\",device=~\"nvme.*\"}[5m])";
          legendFormat = "{{device}} Read";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "-rate(node_disk_written_bytes_total{host=\"$host\",device=~\"nvme.*\"}[5m])";
          legendFormat = "{{device}} Write";
          refId = "B";
        }
      ];
      fieldConfig = {
        defaults = {
          unit = "binBps";
        };
      };
    }
    {
      title = "Filesystem Usage";
      type = "timeseries";
      id = 6;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 16;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "node_filesystem_size_bytes{host=\"$host\",mountpoint=\"/\"} - node_filesystem_free_bytes{host=\"$host\",mountpoint=\"/\"}";
          legendFormat = "Used (/)";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "node_filesystem_free_bytes{host=\"$host\",mountpoint=\"/\"}";
          legendFormat = "Free (/)";
          refId = "B";
        }
      ];
      fieldConfig = {
        defaults = {
          unit = "bytes";
        };
      };
    }
  ];
}
