{
  folder = "Security";
  title = "Authelia";
  uid = "authelia";
  description = "Authentication and authorization metrics for Authelia";
  editable = true;
  schemaVersion = 40;
  refresh = "1m";
  tags = [
    "security"
    "auth"
    "authelia"
  ];
  timezone = "browser";
  time = {
    from = "now-6h";
    to = "now";
  };
  timepicker = { };
  graphTooltip = 1;

  annotations.list = [
    {
      builtIn = 1;
      datasource = {
        type = "grafana";
        uid = "-- Grafana --";
      };
      enable = true;
      hide = true;
      iconColor = "rgba(0, 211, 255, 1)";
      name = "Annotations & Alerts";
      type = "dashboard";
    }
  ];

  templating.list = [ ];

  panels = [

    # ── Overview stats ────────────────────────────────────────────────────

    {
      title = "Auth Requests / s";
      type = "stat";
      id = 1;
      gridPos = {
        h = 4;
        w = 6;
        x = 0;
        y = 0;
      };
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum(rate(authelia_request[5m]))";
          refId = "A";
          instant = true;
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "value";
        graphMode = "area";
      };
      fieldConfig.defaults = {
        unit = "reqps";
        decimals = 2;
        color.mode = "thresholds";
        thresholds = {
          mode = "absolute";
          steps = [
            {
              color = "green";
              value = null;
            }
          ];
        };
      };
    }

    {
      title = "Authz Success Rate";
      type = "stat";
      id = 2;
      gridPos = {
        h = 4;
        w = 6;
        x = 6;
        y = 0;
      };
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum(rate(authelia_authz{code="200"}[5m])) / sum(rate(authelia_authz[5m]))'';
          refId = "A";
          instant = true;
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
        graphMode = "none";
      };
      fieldConfig.defaults = {
        unit = "percentunit";
        decimals = 1;
        min = 0;
        max = 1;
        thresholds = {
          mode = "absolute";
          steps = [
            {
              color = "red";
              value = null;
            }
            {
              color = "yellow";
              value = 0.8;
            }
            {
              color = "green";
              value = 0.95;
            }
          ];
        };
      };
    }

    {
      title = "Authz Denials / s";
      type = "stat";
      id = 3;
      gridPos = {
        h = 4;
        w = 6;
        x = 12;
        y = 0;
      };
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum(rate(authelia_authz{code=~"401|403"}[5m]))'';
          refId = "A";
          instant = true;
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
        graphMode = "area";
      };
      fieldConfig.defaults = {
        unit = "reqps";
        decimals = 2;
        thresholds = {
          mode = "absolute";
          steps = [
            {
              color = "green";
              value = null;
            }
            {
              color = "yellow";
              value = 0.1;
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
      title = "Authn Latency p95";
      type = "stat";
      id = 4;
      gridPos = {
        h = 4;
        w = 6;
        x = 18;
        y = 0;
      };
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "histogram_quantile(0.95, sum(rate(authelia_authn_duration_bucket[5m])) by (le))";
          refId = "A";
          instant = true;
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "value";
        graphMode = "none";
      };
      fieldConfig.defaults = {
        unit = "s";
        decimals = 3;
        thresholds = {
          mode = "absolute";
          steps = [
            {
              color = "green";
              value = null;
            }
            {
              color = "yellow";
              value = 0.5;
            }
            {
              color = "red";
              value = 2;
            }
          ];
        };
      };
    }

    # ── Authorization ─────────────────────────────────────────────────────

    {
      title = "Authorization Requests by Status";
      type = "timeseries";
      id = 5;
      gridPos = {
        h = 8;
        w = 16;
        x = 0;
        y = 4;
      };
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum by (code) (rate(authelia_authz[5m]))";
          legendFormat = "HTTP {{code}}";
          refId = "A";
          range = true;
        }
      ];
      fieldConfig = {
        defaults = {
          unit = "reqps";
          min = 0;
          color.mode = "palette-classic";
          custom = {
            drawStyle = "line";
            lineWidth = 1;
            fillOpacity = 10;
            gradientMode = "none";
            lineInterpolation = "linear";
            showPoints = "auto";
            spanNulls = false;
            stacking = {
              group = "A";
              mode = "none";
            };
            thresholdsStyle.mode = "off";
          };
        };
        overrides = [
          {
            matcher = {
              id = "byRegexp";
              options = "/.*401.*/";
            };
            properties = [
              {
                id = "color";
                value.fixedColor = "orange";
                value.mode = "fixed";
              }
            ];
          }
          {
            matcher = {
              id = "byRegexp";
              options = "/.*403.*/";
            };
            properties = [
              {
                id = "color";
                value.fixedColor = "red";
                value.mode = "fixed";
              }
            ];
          }
          {
            matcher = {
              id = "byRegexp";
              options = "/.*200.*/";
            };
            properties = [
              {
                id = "color";
                value.fixedColor = "green";
                value.mode = "fixed";
              }
            ];
          }
        ];
      };
      options = {
        legend = {
          calcs = [
            "mean"
            "max"
          ];
          displayMode = "table";
          placement = "bottom";
          showLegend = true;
        };
        tooltip = {
          mode = "multi";
          sort = "desc";
        };
      };
    }

    {
      title = "Authz by Status (current)";
      type = "piechart";
      id = 6;
      gridPos = {
        h = 8;
        w = 8;
        x = 16;
        y = 4;
      };
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum by (code) (increase(authelia_authz[$__range]))";
          legendFormat = "HTTP {{code}}";
          refId = "A";
          instant = true;
        }
      ];
      options = {
        pieType = "donut";
        legend = {
          displayMode = "table";
          placement = "bottom";
          showLegend = true;
          values = [
            "value"
            "percent"
          ];
        };
        tooltip = {
          mode = "single";
          sort = "none";
        };
      };
      fieldConfig.defaults = {
        unit = "short";
        color.mode = "palette-classic";
      };
    }

    # ── HTTP Traffic ──────────────────────────────────────────────────────

    {
      title = "HTTP Request Rate by Method";
      type = "timeseries";
      id = 7;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 12;
      };
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum by (method) (rate(authelia_request[5m]))";
          legendFormat = "{{method}}";
          refId = "A";
          range = true;
        }
      ];
      fieldConfig.defaults = {
        unit = "reqps";
        min = 0;
        color.mode = "palette-classic";
        custom = {
          drawStyle = "line";
          lineWidth = 1;
          fillOpacity = 10;
          lineInterpolation = "linear";
          showPoints = "auto";
          spanNulls = false;
          stacking = {
            group = "A";
            mode = "none";
          };
          thresholdsStyle.mode = "off";
        };
      };
      options = {
        legend = {
          calcs = [ "mean" ];
          displayMode = "table";
          placement = "bottom";
          showLegend = true;
        };
        tooltip = {
          mode = "multi";
          sort = "desc";
        };
      };
    }

    {
      title = "HTTP Request Rate by Status";
      type = "timeseries";
      id = 8;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 12;
      };
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum by (code) (rate(authelia_request[5m]))";
          legendFormat = "HTTP {{code}}";
          refId = "A";
          range = true;
        }
      ];
      fieldConfig = {
        defaults = {
          unit = "reqps";
          min = 0;
          color.mode = "palette-classic";
          custom = {
            drawStyle = "line";
            lineWidth = 1;
            fillOpacity = 10;
            lineInterpolation = "linear";
            showPoints = "auto";
            spanNulls = false;
            stacking = {
              group = "A";
              mode = "none";
            };
            thresholdsStyle.mode = "off";
          };
        };
        overrides = [
          {
            matcher = {
              id = "byRegexp";
              options = "/.*5../";
            };
            properties = [
              {
                id = "color";
                value.fixedColor = "red";
                value.mode = "fixed";
              }
            ];
          }
          {
            matcher = {
              id = "byRegexp";
              options = "/.*4../";
            };
            properties = [
              {
                id = "color";
                value.fixedColor = "orange";
                value.mode = "fixed";
              }
            ];
          }
          {
            matcher = {
              id = "byRegexp";
              options = "/.*2../";
            };
            properties = [
              {
                id = "color";
                value.fixedColor = "green";
                value.mode = "fixed";
              }
            ];
          }
        ];
      };
      options = {
        legend = {
          calcs = [ "mean" ];
          displayMode = "table";
          placement = "bottom";
          showLegend = true;
        };
        tooltip = {
          mode = "multi";
          sort = "desc";
        };
      };
    }

    # ── Latency ───────────────────────────────────────────────────────────

    {
      title = "Request Duration Percentiles";
      type = "timeseries";
      id = 9;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 20;
      };
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "histogram_quantile(0.50, sum(rate(authelia_request_duration_bucket[5m])) by (le))";
          legendFormat = "p50";
          refId = "A";
          range = true;
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "histogram_quantile(0.90, sum(rate(authelia_request_duration_bucket[5m])) by (le))";
          legendFormat = "p90";
          refId = "B";
          range = true;
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "histogram_quantile(0.95, sum(rate(authelia_request_duration_bucket[5m])) by (le))";
          legendFormat = "p95";
          refId = "C";
          range = true;
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "histogram_quantile(0.99, sum(rate(authelia_request_duration_bucket[5m])) by (le))";
          legendFormat = "p99";
          refId = "D";
          range = true;
        }
      ];
      fieldConfig.defaults = {
        unit = "s";
        min = 0;
        decimals = 3;
        color.mode = "palette-classic";
        custom = {
          drawStyle = "line";
          lineWidth = 1;
          fillOpacity = 0;
          lineInterpolation = "linear";
          showPoints = "auto";
          spanNulls = false;
          thresholdsStyle.mode = "off";
        };
      };
      options = {
        legend = {
          calcs = [
            "mean"
            "max"
          ];
          displayMode = "table";
          placement = "bottom";
          showLegend = true;
        };
        tooltip = {
          mode = "multi";
          sort = "desc";
        };
      };
    }

    {
      title = "Authn Duration Percentiles";
      type = "timeseries";
      id = 10;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 20;
      };
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "histogram_quantile(0.50, sum(rate(authelia_authn_duration_bucket[5m])) by (le))";
          legendFormat = "p50";
          refId = "A";
          range = true;
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "histogram_quantile(0.90, sum(rate(authelia_authn_duration_bucket[5m])) by (le))";
          legendFormat = "p90";
          refId = "B";
          range = true;
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "histogram_quantile(0.95, sum(rate(authelia_authn_duration_bucket[5m])) by (le))";
          legendFormat = "p95";
          refId = "C";
          range = true;
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "histogram_quantile(0.99, sum(rate(authelia_authn_duration_bucket[5m])) by (le))";
          legendFormat = "p99";
          refId = "D";
          range = true;
        }
      ];
      fieldConfig.defaults = {
        unit = "s";
        min = 0;
        decimals = 3;
        color.mode = "palette-classic";
        custom = {
          drawStyle = "line";
          lineWidth = 1;
          fillOpacity = 0;
          lineInterpolation = "linear";
          showPoints = "auto";
          spanNulls = false;
          thresholdsStyle.mode = "off";
        };
      };
      options = {
        legend = {
          calcs = [
            "mean"
            "max"
          ];
          displayMode = "table";
          placement = "bottom";
          showLegend = true;
        };
        tooltip = {
          mode = "multi";
          sort = "desc";
        };
      };
    }

    # ── Denial heatmap ────────────────────────────────────────────────────

    {
      title = "401 Unauthorized Rate";
      type = "timeseries";
      id = 11;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 28;
      };
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''rate(authelia_authz{code="401"}[5m])'';
          legendFormat = "401 Unauthorized";
          refId = "A";
          range = true;
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''rate(authelia_request{code="401"}[5m])'';
          legendFormat = "401 (all requests)";
          refId = "B";
          range = true;
        }
      ];
      fieldConfig = {
        defaults = {
          unit = "reqps";
          min = 0;
          custom = {
            drawStyle = "line";
            lineWidth = 1;
            fillOpacity = 20;
            lineInterpolation = "linear";
            showPoints = "auto";
            spanNulls = false;
            thresholdsStyle.mode = "off";
          };
          color.mode = "fixed";
          color.fixedColor = "orange";
        };
        overrides = [
          {
            matcher = {
              id = "byName";
              options = "401 (all requests)";
            };
            properties = [
              {
                id = "color";
                value.fixedColor = "semi-dark-orange";
                value.mode = "fixed";
              }
            ];
          }
        ];
      };
      options = {
        legend = {
          calcs = [
            "mean"
            "max"
            "sum"
          ];
          displayMode = "table";
          placement = "bottom";
          showLegend = true;
        };
        tooltip = {
          mode = "multi";
          sort = "desc";
        };
      };
    }

    {
      title = "403 Forbidden Rate";
      type = "timeseries";
      id = 12;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 28;
      };
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''rate(authelia_authz{code="403"}[5m])'';
          legendFormat = "403 Forbidden";
          refId = "A";
          range = true;
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''rate(authelia_request{code="403"}[5m])'';
          legendFormat = "403 (all requests)";
          refId = "B";
          range = true;
        }
      ];
      fieldConfig = {
        defaults = {
          unit = "reqps";
          min = 0;
          custom = {
            drawStyle = "line";
            lineWidth = 1;
            fillOpacity = 20;
            lineInterpolation = "linear";
            showPoints = "auto";
            spanNulls = false;
            thresholdsStyle.mode = "off";
          };
          color.mode = "fixed";
          color.fixedColor = "red";
        };
      };
      options = {
        legend = {
          calcs = [
            "mean"
            "max"
            "sum"
          ];
          displayMode = "table";
          placement = "bottom";
          showLegend = true;
        };
        tooltip = {
          mode = "multi";
          sort = "desc";
        };
      };
    }

    # ── Totals (range) ────────────────────────────────────────────────────

    {
      title = "Total Requests (range)";
      type = "stat";
      id = 13;
      gridPos = {
        h = 4;
        w = 6;
        x = 0;
        y = 36;
      };
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum(increase(authelia_request[$__range]))";
          refId = "A";
          instant = true;
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "value";
        graphMode = "none";
      };
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    {
      title = "Total Authz Checks (range)";
      type = "stat";
      id = 14;
      gridPos = {
        h = 4;
        w = 6;
        x = 6;
        y = 36;
      };
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum(increase(authelia_authz[$__range]))";
          refId = "A";
          instant = true;
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "value";
        graphMode = "none";
      };
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    {
      title = "Total Denials (range)";
      type = "stat";
      id = 15;
      gridPos = {
        h = 4;
        w = 6;
        x = 12;
        y = 36;
      };
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum(increase(authelia_authz{code=~"401|403"}[$__range]))'';
          refId = "A";
          instant = true;
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
        graphMode = "none";
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
    }

    {
      title = "Total Authn Attempts (range)";
      type = "stat";
      id = 16;
      gridPos = {
        h = 4;
        w = 6;
        x = 18;
        y = 36;
      };
      datasource = {
        type = "prometheus";
        uid = "Prometheus";
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum(increase(authelia_authn_duration_count[$__range]))";
          refId = "A";
          instant = true;
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "value";
        graphMode = "none";
      };
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

  ];
}
