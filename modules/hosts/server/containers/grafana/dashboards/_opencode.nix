{
  folder = "Dev";
  title = "OpenCode";
  tags = [
    "dev"
    "ai"
    "opencode"
  ];
  uid = "opencode";
  editable = true;
  schemaVersion = 38;
  refresh = "5m";
  time = {
    from = "now-7d";
    to = "now";
  };
  panels = [

    # ── KPIs ─────────────────────────────────────────────────────────────
    {
      title = "Total Cost";
      type = "stat";
      id = 1;
      gridPos = {
        h = 4;
        w = 4;
        x = 0;
        y = 0;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum(increase(opencode_cost_usage_USD_total[$__range]))";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      options.colorMode = "background";
      fieldConfig.defaults = {
        unit = "currencyUSD";
        decimals = 4;
      };
    }
    {
      title = "Total Tokens";
      type = "stat";
      id = 2;
      gridPos = {
        h = 4;
        w = 4;
        x = 4;
        y = 0;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum(increase(opencode_token_usage_tokens_total[$__range]))";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      options.colorMode = "background";
      fieldConfig.defaults.unit = "short";
    }
    {
      title = "Sessions";
      type = "stat";
      id = 3;
      gridPos = {
        h = 4;
        w = 4;
        x = 8;
        y = 0;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum(increase(opencode_session_count_total[$__range]))";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      options.colorMode = "background";
      fieldConfig.defaults.unit = "short";
    }
    {
      title = "Messages";
      type = "stat";
      id = 4;
      gridPos = {
        h = 4;
        w = 4;
        x = 12;
        y = 0;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum(increase(opencode_message_count_total[$__range]))";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      options.colorMode = "background";
      fieldConfig.defaults.unit = "short";
    }
    {
      title = "Commits";
      type = "stat";
      id = 5;
      gridPos = {
        h = 4;
        w = 4;
        x = 16;
        y = 0;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum(increase(opencode_commit_count_total[$__range]))";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      options.colorMode = "background";
      fieldConfig.defaults.unit = "short";
    }
    {
      title = "Avg Cost / Session";
      type = "stat";
      id = 6;
      gridPos = {
        h = 4;
        w = 4;
        x = 20;
        y = 0;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum(opencode_session_cost_total_USD_sum) / sum(opencode_session_cost_total_USD_count)";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      options.colorMode = "background";
      fieldConfig.defaults = {
        unit = "currencyUSD";
        decimals = 4;
      };
    }

    # ── Cost & Tokens ─────────────────────────────────────────────────────
    {
      title = "Cost by Model";
      type = "timeseries";
      id = 7;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 4;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum by (model) (rate(opencode_cost_usage_USD_total[$__rate_interval]))";
          legendFormat = "{{model}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "currencyUSD";
        decimals = 6;
      };
    }
    {
      title = "Token Usage by Type";
      type = "timeseries";
      id = 8;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 4;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum by (type) (rate(opencode_token_usage_tokens_total[$__rate_interval]))";
          legendFormat = "{{type}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "short";
    }

    # ── Model & Retries ───────────────────────────────────────────────────
    {
      title = "Model Usage";
      type = "piechart";
      id = 9;
      gridPos = {
        h = 8;
        w = 8;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum by (model) (increase(opencode_model_usage_total[$__range]))";
          legendFormat = "{{model}}";
          refId = "A";
        }
      ];
      options.displayLabels = [
        "percent"
        "name"
      ];
      fieldConfig.defaults.unit = "short";
    }
    {
      title = "Cache Activity";
      type = "timeseries";
      id = 10;
      gridPos = {
        h = 8;
        w = 8;
        x = 8;
        y = 12;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum by (type) (rate(opencode_cache_count_total[$__rate_interval]))";
          legendFormat = "{{type}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "short";
    }
    {
      title = "API Retry Rate";
      type = "timeseries";
      id = 11;
      gridPos = {
        h = 8;
        w = 8;
        x = 16;
        y = 12;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "sum(rate(opencode_retry_count_total[$__rate_interval]))";
          legendFormat = "retries/s";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "short";
    }

    # ── Lines of Code ─────────────────────────────────────────────────────
    {
      title = "Lines of Code (live cumulative)";
      type = "timeseries";
      id = 12;
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
          expr = "sum by (type) (opencode_lines_of_code_total)";
          legendFormat = "{{type}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "short";
    }
    {
      title = "Lines of Code (gross churn)";
      type = "timeseries";
      id = 13;
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
          expr = "sum by (type) (increase(opencode_lines_of_code_count_total[$__rate_interval]))";
          legendFormat = "{{type}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "short";
    }

    # ── Session & Tool Performance ────────────────────────────────────────
    {
      title = "Session Duration";
      type = "timeseries";
      id = 14;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 28;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "histogram_quantile(0.5, sum by (le) (rate(opencode_session_duration_milliseconds_bucket[$__rate_interval])))";
          legendFormat = "p50";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "histogram_quantile(0.95, sum by (le) (rate(opencode_session_duration_milliseconds_bucket[$__rate_interval])))";
          legendFormat = "p95";
          refId = "B";
        }
      ];
      fieldConfig.defaults.unit = "ms";
    }
    {
      title = "Tool Duration";
      type = "timeseries";
      id = 15;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 28;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "histogram_quantile(0.5, sum by (le) (rate(opencode_tool_duration_milliseconds_bucket[$__rate_interval])))";
          legendFormat = "p50";
          refId = "A";
        }
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = "histogram_quantile(0.95, sum by (le) (rate(opencode_tool_duration_milliseconds_bucket[$__rate_interval])))";
          legendFormat = "p95";
          refId = "B";
        }
      ];
      fieldConfig.defaults.unit = "ms";
    }

  ];
}
