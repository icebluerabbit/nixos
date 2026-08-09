let
  prom = {
    type = "prometheus";
    uid = "Prometheus";
  };
  loki = {
    type = "loki";
    uid = "Loki";
  };

  owner = ''owner="slskd"'';
  vpngw = ''host="vpn-gw"'';

  tsDefaults = {
    color.mode = "palette-classic";
    custom = {
      axisBorderShow = false;
      axisCenteredZero = false;
      axisColorMode = "text";
      axisPlacement = "auto";
      barAlignment = 0;
      drawStyle = "line";
      fillOpacity = 10;
      gradientMode = "none";
      hideFrom = {
        legend = false;
        tooltip = false;
        viz = false;
      };
      insertNulls = false;
      lineInterpolation = "linear";
      lineWidth = 1;
      pointSize = 5;
      scaleDistribution.type = "linear";
      showPoints = "auto";
      spanNulls = false;
      stacking = {
        group = "A";
        mode = "none";
      };
      thresholdsStyle.mode = "off";
    };
    mappings = [ ];
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

  tsOptions = {
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
      maxHeight = 600;
      mode = "multi";
      sort = "desc";
    };
  };

  targetInstant = expr: legendFormat: refId: {
    datasource = prom;
    editorMode = "code";
    exemplar = false;
    inherit expr legendFormat refId;
    instant = true;
    range = false;
  };

  target = expr: legendFormat: refId: {
    datasource = prom;
    editorMode = "code";
    inherit expr legendFormat refId;
    instant = false;
    range = true;
  };

  stat =
    {
      id,
      title,
      x,
      y,
      w,
      h ? 4,
      expr,
      unit ? "none",
      legendFormat ? "__auto",
      textMode ? "auto",
      colorMode ? "value",
      graphMode ? "none",
      thresholds ? [
        {
          color = "green";
          value = null;
        }
      ],
    }:
    {
      inherit id title;
      type = "stat";
      pluginVersion = "11.4.0";
      gridPos = {
        inherit
          x
          y
          w
          h
          ;
      };
      datasource = prom;
      fieldConfig = {
        defaults = {
          color.mode = "thresholds";
          mappings = [ ];
          inherit unit;
          thresholds = {
            mode = "absolute";
            steps = thresholds;
          };
        };
        overrides = [ ];
      };
      options = {
        inherit colorMode graphMode textMode;
        justifyMode = "center";
        orientation = "auto";
        reduceOptions = {
          calcs = [ "lastNotNull" ];
          fields = "";
          values = false;
        };
        wideLayout = true;
      };
      targets = [ (targetInstant expr legendFormat "A") ];
    };

  ts =
    {
      id,
      title,
      x,
      y,
      w,
      h ? 8,
      unit ? "Bps",
      targets,
      description ? "",
    }:
    {
      inherit
        id
        title
        targets
        description
        ;
      type = "timeseries";
      pluginVersion = "11.4.0";
      gridPos = {
        inherit
          x
          y
          w
          h
          ;
      };
      datasource = prom;
      fieldConfig = {
        defaults = tsDefaults // {
          inherit unit;
        };
        overrides = [ ];
      };
      options = tsOptions;
    };

  # slskd's traffic rides its currently-claimed tunnel; join the vpn-gw wireguard counters
  # (labelled `interface`) to slskd's claim (labelled `tunnel`) to isolate slskd's tunnel.
  claimedRate =
    metric:
    "rate(${metric}{${vpngw}}[$__rate_interval]) * on(interface) group_left() "
    + ''label_replace(vpngw_tunnel_claimed{${owner}}, "interface", "$1", "tunnel", "(.*)")'';
in
{
  folder = "Media";
  title = "Soulseek (slskd)";
  uid = "soulseek7c4b1a9e2f6d";
  description = "slskd Soulseek client behind the vpn-gw tunnel broker: claimed tunnel / forwarded port / exit IP (broker /export), VPN-isolated throughput (wireguard-exporter), distributed-network reach + incoming-search serving (slskd /metrics), process health (prometheus-net), and logs (Loki).";
  editable = true;
  graphTooltip = 1;
  refresh = "30s";
  schemaVersion = 40;
  tags = [
    "media"
    "soulseek"
    "slskd"
    "vpn"
  ];
  timezone = "browser";
  time = {
    from = "now-6h";
    to = "now";
  };
  templating.list = [ ];
  annotations.list = [ ];

  panels = [
    # ── Row 1: broker/VPN + process headline ─────────────────────────────
    (stat {
      id = 1;
      title = "VPN Tunnel";
      x = 0;
      y = 0;
      w = 4;
      expr = "vpngw_tunnel_claimed{${owner}}";
      legendFormat = "{{tunnel}}";
      textMode = "name";
      colorMode = "none";
    })
    (stat {
      id = 2;
      title = "Fwd Port";
      x = 4;
      y = 0;
      w = 4;
      expr = "vpngw_tunnel_forwarded_port{${owner}}";
    })
    (stat {
      id = 3;
      title = "Exit IP";
      x = 8;
      y = 0;
      w = 5;
      expr = "max by (exit_ip) (vpngw_tunnel_exit_ip{${vpngw}} * on(tunnel) group_left() vpngw_tunnel_claimed{${owner}})";
      legendFormat = "{{exit_ip}}";
      textMode = "name";
      colorMode = "none";
    })
    (stat {
      id = 4;
      title = "Tunnel Healthy";
      x = 13;
      y = 0;
      w = 4;
      expr = "max(vpngw_tunnel_healthy{${vpngw}} * on(tunnel) group_left() vpngw_tunnel_claimed{${owner}})";
      colorMode = "background";
      thresholds = [
        {
          color = "red";
          value = null;
        }
        {
          color = "green";
          value = 1;
        }
      ];
    })
    (stat {
      id = 5;
      title = "Memory";
      x = 17;
      y = 0;
      w = 4;
      # prometheus-net exposes working-set (not resident_memory) for the .NET process.
      expr = ''process_working_set_bytes{host="slskd"}'';
      unit = "bytes";
      graphMode = "area";
    })
    (stat {
      id = 6;
      title = "Uptime";
      x = 21;
      y = 0;
      w = 3;
      expr = ''time() - process_start_time_seconds{host="slskd"}'';
      unit = "s";
    })

    # ── Row 2: tunnel throughput (slskd's claimed tunnel) ─────────────────
    (ts {
      id = 10;
      title = "Download (from peers, via VPN)";
      description = "Decrypted inbound on slskd's claimed wg tunnel — Soulseek downloads egressing the VPN.";
      x = 0;
      y = 4;
      w = 12;
      unit = "Bps";
      targets = [ (target (claimedRate "wireguard_received_bytes_total") "{{interface}} RX" "A") ];
    })
    (ts {
      id = 11;
      title = "Upload (to peers, via VPN)";
      description = "Encrypted outbound on slskd's claimed wg tunnel — Soulseek uploads/sharing.";
      x = 12;
      y = 4;
      w = 12;
      unit = "Bps";
      targets = [ (target (claimedRate "wireguard_sent_bytes_total") "{{interface}} TX" "A") ];
    })

    # ── Row 3: distributed network + search serving (seeding reach) ───────
    (ts {
      id = 20;
      title = "Distributed Network";
      description = "Children the node fans searches out to (vs child_limit=256) and whether a distributed parent is attached. More children = the shared library reaches more searchers = more upload demand.";
      x = 0;
      y = 12;
      w = 8;
      unit = "none";
      targets = [
        (target ''slskd_dnet_children{host="slskd"}'' "children" "A")
        (target ''slskd_dnet_child_limit{host="slskd"}'' "child limit" "B")
        (target ''slskd_dnet_has_parent{host="slskd"}'' "has parent" "C")
      ];
    })
    (ts {
      id = 21;
      title = "Incoming Searches /min";
      description = "Searches received vs answered (we hold a matching file → an upload candidate) vs dropped (shed once concurrency=100/circuit_breaker=2000 is exceeded). Sustained drops → raise the throttle.";
      x = 8;
      y = 12;
      w = 8;
      unit = "none";
      targets = [
        (target ''slskd_search_incoming_request_receive_rate_current{host="slskd"}'' "received" "A")
        (target ''slskd_search_incoming_response_send_rate_current{host="slskd"}'' "answered" "B")
        (target ''slskd_search_incoming_request_drop_rate_current{host="slskd"}'' "dropped" "C")
      ];
    })
    (ts {
      id = 22;
      title = "Search Latency";
      description = "Time to serve an incoming search: share-DB query, filter, and total response latency (memory share cache keeps query low).";
      x = 16;
      y = 12;
      w = 8;
      unit = "ms";
      targets = [
        (target ''slskd_search_incoming_query_latency_current{host="slskd"}'' "query" "A")
        (target ''slskd_search_incoming_filter_latency_current{host="slskd"}'' "filter" "B")
        (target ''slskd_search_incoming_response_latency_current{host="slskd"}'' "response" "C")
      ];
    })

    # ── Row 4: peer connections + app socket throughput + CPU ─────────────
    (ts {
      id = 30;
      title = "Peer Connections";
      description = "Established TCP sockets (all .NET sockets — Soulseek peers dominate; a few are web/API). Outbound = peers we dial; inbound = peers dialing our forwarded port.";
      x = 0;
      y = 20;
      w = 8;
      unit = "none";
      targets = [
        (target ''system_net_sockets_outgoing_connections_established{host="slskd"}'' "outgoing" "A")
        (target ''system_net_sockets_incoming_connections_established{host="slskd"}'' "incoming" "B")
      ];
    })
    (ts {
      id = 31;
      title = "Socket Throughput (app)";
      description = "App-level bytes across all .NET sockets (includes web/API, but Soulseek transfers dominate). The VPN-isolated figure is Row 2; this is the whole process for cross-check.";
      x = 8;
      y = 20;
      w = 8;
      unit = "Bps";
      targets = [
        (target ''rate(dotnet_sockets_bytes_received_total{host="slskd"}[$__rate_interval])'' "RX" "A")
        (target ''rate(dotnet_sockets_bytes_sent_total{host="slskd"}[$__rate_interval])'' "TX" "B")
      ];
    })
    (ts {
      id = 32;
      title = "Process CPU";
      description = "prometheus-net process CPU. Share scans + transfers are the main load.";
      x = 16;
      y = 20;
      w = 8;
      unit = "percent";
      targets = [
        (target ''100 * rate(process_cpu_seconds_total{host="slskd"}[$__rate_interval])'' "CPU %" "A")
      ];
    })

    # ── Row 5: logs ───────────────────────────────────────────────────────
    {
      id = 40;
      type = "logs";
      title = "slskd Logs";
      description = "Container journal via the Vector sidecar → Loki.";
      pluginVersion = "11.4.0";
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 28;
      };
      datasource = loki;
      options = {
        dedupStrategy = "none";
        enableLogDetails = true;
        prettifyLogMessage = false;
        showCommonLabels = false;
        showLabels = false;
        showTime = true;
        sortOrder = "Descending";
        wrapLogMessage = false;
      };
      targets = [
        {
          datasource = loki;
          editorMode = "code";
          expr = ''{host="slskd"}'';
          queryType = "range";
          refId = "A";
        }
      ];
    }
  ];
}
