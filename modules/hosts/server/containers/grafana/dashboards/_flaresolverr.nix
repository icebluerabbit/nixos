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
  title = "FlareSolverr";
  tags = [
    "media"
    "proxy"
    "flaresolverr"
  ];
  uid = "flaresolverr";
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
      title = "Requests";
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
          expr = "sum(increase(flaresolverr_requests_total[$__range]))";
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
      title = "Challenges Solved";
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
          expr = ''sum(increase(flaresolverr_challenge_total{result="solved"}[$__range]))'';
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
      title = "Avg Response";
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
          expr = "avg_over_time(flaresolverr_response_seconds[$__range])";
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "value";
        graphMode = "area";
      };
      fieldConfig.defaults = {
        unit = "s";
        decimals = 2;
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
          expr = "sum(increase(flaresolverr_errors_total[$__range]))";
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

    # ── Request rate ─────────────────────────────────────────────────────
    {
      title = "Request Rate by Status";
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
          expr = "sum by (status) (rate(flaresolverr_requests_total[5m]))";
          legendFormat = "{{status}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "reqps";
    }
    {
      title = "Response Time";
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
          expr = "flaresolverr_response_seconds";
          legendFormat = "solve time";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "s";
    }

    # ── Challenge outcomes ───────────────────────────────────────────────
    {
      title = "Challenge Outcomes";
      type = "piechart";
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
          expr = "sum by (result) (increase(flaresolverr_challenge_total[$__range]))";
          legendFormat = "{{result}}";
          refId = "A";
        }
      ];
      options.displayLabels = [
        "name"
        "percent"
      ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Requests by Method";
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
          expr = "sum by (method) (rate(flaresolverr_requests_total[5m]))";
          legendFormat = "{{method}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "reqps";
    }

    # ── Logs ─────────────────────────────────────────────────────────────
    {
      title = "Recent Activity";
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
          expr = ''{host="flaresolverr", host_type="container"} |~ "POST /v1|Challenge|ERROR"'';
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
