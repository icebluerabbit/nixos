{
  folder = "Network";
  title = "Network Metrics";
  tags = [
    "network"
    "metrics"
  ];
  uid = "network-metrics";
  editable = true;
  schemaVersion = 38;
  refresh = "10s";
  time = {
    from = "now-1h";
    to = "now";
  };
  templating.list = [
    {
      name = "device";
      type = "query";
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      query = "label_values(unpoller_device_port_receive_bytes_total{host_type=\"network\"}, device)";
      refresh = 1;
      includeAll = true;
      multi = true;
      current = { };
    }
  ];
  panels = [
    {
      title = "Internet Throughput (WAN)";
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
          expr = "sum(unpoller_device_wan_receive_rate_bytes{host_type=\"network\"})";
          legendFormat = "Download";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum(unpoller_device_wan_transmit_rate_bytes{host_type=\"network\"})";
          legendFormat = "Upload";
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
      title = "WAN Latency";
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
          expr = "unpoller_site_latency_seconds{host_type=\"network\"} * 1000";
          legendFormat = "Ping Latency";
          refId = "A";
        }
      ];
      fieldConfig = {
        defaults = {
          unit = "ms";
        };
      };
    }
    {
      title = "WiFi Clients by Band";
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
          expr = "sum by (band) (unpoller_topology_connections_by_band{host_type=\"network\"})";
          legendFormat = "{{band}}";
          refId = "A";
        }
      ];
      fieldConfig = {
        defaults = {
          unit = "short";
        };
      };
    }
    {
      title = "Total Clients";
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
          expr = "unpoller_topology_connections_wired{host_type=\"network\"}";
          legendFormat = "Wired";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "unpoller_topology_connections_wireless{host_type=\"network\"}";
          legendFormat = "Wireless";
          refId = "B";
        }
      ];
      fieldConfig = {
        defaults = {
          unit = "short";
        };
      };
    }
    {
      title = "Switch Port Traffic";
      type = "timeseries";
      id = 5;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 16;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "rate(unpoller_device_port_receive_bytes_total{host_type=\"network\",device=~\"$device\"}[5m])";
          legendFormat = "{{device}} Port {{port}} Rx";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "-rate(unpoller_device_port_transmit_bytes_total{host_type=\"network\",device=~\"$device\"}[5m])";
          legendFormat = "{{device}} Port {{port}} Tx";
          refId = "B";
        }
      ];
      fieldConfig = {
        defaults = {
          unit = "binBps";
        };
      };
    }
  ];
}
