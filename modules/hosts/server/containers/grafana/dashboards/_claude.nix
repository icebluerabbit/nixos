{
  folder = "Dev";
  title = "Claude Code";
  tags = [
    "dev"
    "ai"
    "claude"
  ];
  uid = "claude-code";
  editable = true;
  schemaVersion = 38;
  refresh = "30m";
  time = {
    from = "now-7d";
    to = "now";
  };
  panels = [

    # ── KPIs ─────────────────────────────────────────────────────────
    {
      title = "Total Cost";
      type = "stat";
      id = 1;
      gridPos = {
        h = 4;
        w = 5;
        x = 0;
        y = 0;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum(increase({__name__="claude_code.cost.usage"}[$__range]))'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
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
        w = 5;
        x = 5;
        y = 0;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum(increase({__name__="claude_code.token.usage"}[$__range]))'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults.unit = "short";
    }
    {
      title = "Active Time";
      type = "stat";
      id = 3;
      gridPos = {
        h = 4;
        w = 5;
        x = 10;
        y = 0;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum(increase({__name__="claude_code.active_time.total"}[$__range]))'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults.unit = "s";
    }
    {
      title = "Lines Added";
      type = "stat";
      id = 4;
      gridPos = {
        h = 4;
        w = 5;
        x = 15;
        y = 0;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum({__name__="claude_code.lines_of_code.count", type="added"})'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults.unit = "short";
    }
    {
      title = "Edits Accepted";
      type = "stat";
      id = 5;
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
          expr = ''sum({__name__="claude_code.code_edit_tool.decision", decision="accept"})'';
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults.unit = "short";
    }

    # ── Cost & Token Trends ───────────────────────────────────────────
    {
      title = "Cost by Model";
      type = "timeseries";
      id = 6;
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
          expr = ''sum by (model) (rate({__name__="claude_code.cost.usage"}[$__rate_interval]))'';
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
      id = 7;
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
          expr = ''sum by (type) (rate({__name__="claude_code.token.usage"}[$__rate_interval]))'';
          legendFormat = "{{type}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "short";
    }

    # ── Model Breakdown ───────────────────────────────────────────────
    {
      title = "Tokens by Model";
      type = "piechart";
      id = 8;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum by (model) (increase({__name__="claude_code.token.usage"}[$__range]))'';
          legendFormat = "{{model}}";
          refId = "A";
        }
      ];
      options.displayLabels = [ "percent" ];
      fieldConfig.defaults.unit = "short";
    }
    {
      title = "Cost by Model";
      type = "piechart";
      id = 9;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 12;
      };
      targets = [
        {
          datasource = {
            type = "prometheus";
            uid = "Prometheus";
          };
          expr = ''sum by (model) (increase({__name__="claude_code.cost.usage"}[$__range]))'';
          legendFormat = "{{model}}";
          refId = "A";
        }
      ];
      options.displayLabels = [ "percent" ];
      fieldConfig.defaults = {
        unit = "currencyUSD";
        decimals = 4;
      };
    }

    # ── Productivity ──────────────────────────────────────────────────
    {
      title = "Lines of Code";
      type = "timeseries";
      id = 10;
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
          expr = ''sum by (type) (increase({__name__="claude_code.lines_of_code.count"}[30m]))'';
          legendFormat = "{{type}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults.unit = "short";
    }
    {
      title = "Code Edit Acceptance Rate";
      type = "timeseries";
      id = 11;
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
          expr = ''sum({__name__="claude_code.code_edit_tool.decision", decision="accept"}) / sum({__name__="claude_code.code_edit_tool.decision"})'';
          legendFormat = "acceptance rate";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "percentunit";
        min = 0;
        max = 1;
      };
    }

  ];
}
