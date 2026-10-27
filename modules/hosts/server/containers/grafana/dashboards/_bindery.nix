# Bindery exposes a native Prometheus registry at /metrics (router root, no auth
# — upstream's deliberate choice since scrapes carry no session cookie), scraped
# by the Vector sidecar. Series available: bindery_http_requests_total{method,
# route,status}, bindery_http_request_duration_seconds{method,route},
# bindery_scheduler_runs_total{job,result}, bindery_scheduler_run_duration_seconds
# {job}, bindery_build_info{version,commit,date}. Library/queue counts are NOT
# exposed — those panels would need the API, not metrics.
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
  title = "Bindery";
  tags = [
    "media"
    "books"
    "bindery"
  ];
  uid = "bindery";
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
      title = "Version";
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
          expr = "bindery_build_info";
          format = "table";
          instant = true;
          refId = "A";
        }
      ];
      options = {
        reduceOptions = {
          calcs = [ "lastNotNull" ];
          fields = "/^version$/";
        };
        colorMode = "background";
        textMode = "value";
      };
    }
    {
      title = "Requests (24h)";
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
          expr = "sum(increase(bindery_http_requests_total[24h]))";
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
      title = "5xx (24h)";
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
          expr = ''sum(increase(bindery_http_requests_total{status=~"5.."}[24h]))'';
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
      title = "Scheduler Panics (24h)";
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
          expr = ''sum(increase(bindery_scheduler_runs_total{result="panic"}[24h]))'';
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

    # ── Scheduler ────────────────────────────────────────────────────────
    {
      title = "Background Jobs by Result";
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
          expr = "sum by (job, result) (rate(bindery_scheduler_runs_total[5m]))";
          legendFormat = "{{job}} · {{result}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "reqps";
    }
    {
      title = "Job Duration (p95)";
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
          expr = "histogram_quantile(0.95, sum by (job, le) (rate(bindery_scheduler_run_duration_seconds_bucket[5m])))";
          legendFormat = "{{job}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "s";
    }

    # ── HTTP ─────────────────────────────────────────────────────────────
    {
      title = "Requests by Route";
      type = "timeseries";
      id = 7;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "sum by (route) (rate(bindery_http_requests_total[5m]))";
          legendFormat = "{{route}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "reqps";
    }
    {
      title = "Request Duration (p95)";
      type = "timeseries";
      id = 8;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "histogram_quantile(0.95, sum by (route, le) (rate(bindery_http_request_duration_seconds_bucket[5m])))";
          legendFormat = "{{route}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "s";
    }

    # ── Logs ─────────────────────────────────────────────────────────────
    {
      title = "Warnings & Errors";
      type = "logs";
      id = 9;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 20;
      };
      targets = [
        {
          datasource = loki;
          # Bindery logs structured JSON via slog, so match the level field.
          expr = ''{host="bindery", host_type="container"} |~ "\"level\":\"(WARN|ERROR)\""'';
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
      id = 10;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 28;
      };
      targets = [
        {
          datasource = loki;
          expr = ''{host="bindery", host_type="container"}'';
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
