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
  title = "dnsmasq";
  uid = "dnsmasq";
  tags = [
    "network"
    "dns"
    "dnsmasq"
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
      title = "Cache Size";
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
          expr = "dnsmasq_cachesize";
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
      title = "Cache Hit Ratio";
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
          expr = "100 * increase(dnsmasq_hits[$__range]) / clamp_min(increase(dnsmasq_hits[$__range]) + increase(dnsmasq_misses[$__range]), 1)";
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
      title = "Hits (range)";
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
          expr = "increase(dnsmasq_hits[$__range])";
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
      title = "Misses (range)";
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
          expr = "increase(dnsmasq_misses[$__range])";
          refId = "A";
        }
      ];
      options.reduceOptions.calcs = [ "lastNotNull" ];
      fieldConfig.defaults = {
        unit = "short";
        decimals = 0;
      };
    }

    # ── Query rates ───────────────────────────────────────────────────────
    {
      title = "Cache Hits / Misses (per sec)";
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
          expr = "rate(dnsmasq_hits[5m])";
          legendFormat = "hits";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "rate(dnsmasq_misses[5m])";
          legendFormat = "misses";
          refId = "B";
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
      title = "Cache Insertions / Evictions (per sec)";
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
          expr = "rate(dnsmasq_insertions[5m])";
          legendFormat = "insertions";
          refId = "A";
        }
        {
          datasource = prom;
          expr = "rate(dnsmasq_evictions[5m])";
          legendFormat = "evictions";
          refId = "B";
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

    # ── Per-upstream ──────────────────────────────────────────────────────
    {
      title = "Queries by Upstream Server";
      type = "timeseries";
      id = 7;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 12;
      };
      targets = [
        {
          datasource = prom;
          expr = "rate(dnsmasq_servers_queries[5m])";
          legendFormat = "{{server}}";
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

    # ── Logs ──────────────────────────────────────────────────────────────
    {
      title = "dnsmasq Logs";
      type = "logs";
      id = 8;
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 20;
      };
      targets = [
        {
          datasource = loki;
          expr = ''{host="dnsmasq", host_type="container"}'';
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
