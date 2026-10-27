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
  folder = "Network";
  title = "Pi-hole";
  uid = "pi-hole";
  tags = [
    "network"
    "dns"
    "pi-hole"
  ];
  editable = true;
  schemaVersion = 38;
  refresh = "30s";
  time = {
    from = "now-6h";
    to = "now";
  };
  panels = [

    # ── KPIs ──────────────────────────────────────────────────────────────
    {
      title = "FTL Status";
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
          datasource = prom;
          expr = "pihole_status";
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults = {
        mappings = [
          {
            type = "value";
            options = {
              "0".text = "disabled";
              "1".text = "enabled";
            };
          }
        ];
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
    }
    {
      title = "Blocked Today";
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
          datasource = prom;
          expr = "pihole_ads_percentage_today";
          refId = "A";
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
      };
      fieldConfig.defaults = {
        unit = "percent";
        min = 0;
        max = 100;
        decimals = 1;
      };
    }
    {
      title = "Queries Today";
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
          datasource = prom;
          expr = "pihole_dns_queries_today";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Ads Blocked Today";
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
          datasource = prom;
          expr = "pihole_ads_blocked_today";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Gravity Domains";
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
          datasource = prom;
          expr = "pihole_domains_being_blocked";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Unique Clients";
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
          datasource = prom;
          expr = "pihole_unique_clients";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    # ── Query flow ────────────────────────────────────────────────────────
    {
      title = "Query Rate";
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
          datasource = prom;
          expr = "pihole_request_rate";
          legendFormat = "queries";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "qps";
        custom = {
          drawStyle = "line";
          fillOpacity = 10;
          lineWidth = 1;
        };
      };
    }
    {
      title = "Cached vs Forwarded vs Blocked";
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
          datasource = prom;
          expr = "pihole_queries_cached";
          legendFormat = "cached";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "pihole_queries_forwarded";
          legendFormat = "forwarded";
          refId = "B";
        }
        {
          datasource = prom;
          expr = "pihole_ads_blocked_today";
          legendFormat = "blocked";
          refId = "C";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        custom = {
          drawStyle = "line";
          fillOpacity = 10;
          lineWidth = 1;
          stacking.mode = "normal";
        };
      };
    }

    # ── Split verification ────────────────────────────────────────────────
    # The whole point of the routing design: internal domains must land on the
    # dnsmasq container and public ones on Cloudflare. A destination drifting to
    # zero here is the split silently breaking.
    {
      title = "Queries by Forward Destination";
      type = "timeseries";
      id = 9;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "pihole_forward_destinations";
          legendFormat = "{{destination}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        custom = {
          drawStyle = "line";
          fillOpacity = 10;
          lineWidth = 1;
        };
      };
    }
    {
      title = "Upstream Response Time";
      type = "timeseries";
      id = 10;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "pihole_forward_destinations_responsetime";
          legendFormat = "{{destination}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "ms";
        custom = {
          drawStyle = "line";
          fillOpacity = 10;
          lineWidth = 1;
        };
      };
    }

    # ── Breakdowns ────────────────────────────────────────────────────────
    {
      title = "Query Types";
      type = "timeseries";
      id = 11;
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 20;
      };
      targets = [
        {
          datasource = prom;
          expr = "pihole_querytypes";
          legendFormat = "{{type}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "percent";
        custom = {
          drawStyle = "line";
          fillOpacity = 10;
          lineWidth = 1;
        };
      };
    }
    {
      title = "Reply Types";
      type = "timeseries";
      id = 12;
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 20;
      };
      targets = [
        {
          datasource = prom;
          expr = "pihole_reply";
          legendFormat = "{{type}}";
          refId = "A";
        }
      ];
      fieldConfig.defaults = {
        unit = "short";
        custom = {
          drawStyle = "line";
          fillOpacity = 10;
          lineWidth = 1;
        };
      };
    }

    # ── Tops ──────────────────────────────────────────────────────────────
    {
      title = "Top Blocked Domains";
      type = "bargauge";
      id = 13;
      gridPos = {
        h = 9;
        w = 8;
        x = 0;
        y = 28;
      };
      targets = [
        {
          datasource = prom;
          expr = "topk(10, pihole_top_ads)";
          legendFormat = "{{domain}}";
          refId = "A";
        }
      ];
      options = {
        displayMode = "gradient";
        orientation = "horizontal";
        reduceOptions.calcs = [ "lastNotNull" ];
      };
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Top Permitted Domains";
      type = "bargauge";
      id = 14;
      gridPos = {
        h = 9;
        w = 8;
        x = 8;
        y = 28;
      };
      targets = [
        {
          datasource = prom;
          expr = "topk(10, pihole_top_queries)";
          legendFormat = "{{domain}}";
          refId = "A";
        }
      ];
      options = {
        displayMode = "gradient";
        orientation = "horizontal";
        reduceOptions.calcs = [ "lastNotNull" ];
      };
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }
    {
      title = "Top Clients";
      type = "bargauge";
      id = 15;
      gridPos = {
        h = 9;
        w = 8;
        x = 16;
        y = 28;
      };
      targets = [
        {
          datasource = prom;
          expr = "topk(10, pihole_top_sources)";
          legendFormat = "{{source}}";
          refId = "A";
        }
      ];
      options = {
        displayMode = "gradient";
        orientation = "horizontal";
        reduceOptions.calcs = [ "lastNotNull" ];
      };
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    # ── Logs ──────────────────────────────────────────────────────────────
    {
      title = "Pi-hole Logs";
      type = "logs";
      id = 16;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 37;
      };
      targets = [
        {
          datasource = loki;
          expr = ''{host="pi-hole", host_type="container"}'';
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
