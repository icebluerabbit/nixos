{
  folder = "Infrastructure";
  title = "Container Metrics";
  tags = [
    "infra"
    "metrics"
    "container"
  ];
  uid = "container-metrics";
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
      title = "CPU Usage per Container";
      type = "timeseries";
      id = 1;
      gridPos = {
        h = 10;
        w = 24;
        x = 0;
        y = 0;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum by (container) (label_replace(rate(namedprocess_namegroup_cpu_seconds_total{host=\"$host\",groupname=~\".*/container@[^.]+\\\\.service.*\"}[5m]),\"container\",\"$1\",\"groupname\",\".*/container@([^.]+)\\\\.service.*\")) * 100";
          legendFormat = "{{container}}";
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
      title = "Memory Usage per Container";
      type = "timeseries";
      id = 2;
      gridPos = {
        h = 10;
        w = 24;
        x = 0;
        y = 10;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum by (container) (label_replace(namedprocess_namegroup_memory_bytes{host=\"$host\",groupname=~\".*/container@[^.]+\\\\.service.*\",memtype=\"resident\"},\"container\",\"$1\",\"groupname\",\".*/container@([^.]+)\\\\.service.*\"))";
          legendFormat = "{{container}}";
          refId = "A";
        }
      ];
      fieldConfig = {
        defaults = {
          unit = "bytes";
        };
      };
    }
    {
      title = "Container Thread Count";
      type = "timeseries";
      id = 3;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 20;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "systemd_unit_tasks_current{host=\"$host\",name=~\"container@.*\"}";
          legendFormat = "{{name}}";
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
      title = "Network Traffic per Container";
      type = "timeseries";
      id = 4;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 20;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "rate(systemd_service_ip_egress_bytes{host=\"$host\",name=~\"container@.*\"}[5m])";
          legendFormat = "{{name}} Out";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "rate(systemd_service_ip_ingress_bytes{host=\"$host\",name=~\"container@.*\"}[5m])";
          legendFormat = "{{name}} In";
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
