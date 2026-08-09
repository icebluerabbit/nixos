let
  prom = {
    type = "prometheus";
    uid = "Prometheus";
  };

  host = ''host="qbittorrent"'';

  tsDefaults = {
    color.mode = "palette-classic";
    custom = {
      axisBorderShow = false;
      axisCenteredZero = false;
      axisColorMode = "text";
      axisLabel = "";
      axisPlacement = "auto";
      barAlignment = 0;
      barWidthFactor = 0.6;
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

  target = expr: legendFormat: refId: {
    datasource = prom;
    editorMode = "code";
    inherit expr legendFormat refId;
    instant = false;
    range = true;
  };

  targetInstant = expr: legendFormat: refId: {
    datasource = prom;
    editorMode = "code";
    exemplar = false;
    inherit expr legendFormat refId;
    instant = true;
    range = false;
  };

  # table query (instant, table-formatted)
  tableTarget = expr: refId: {
    datasource = prom;
    editorMode = "code";
    inherit expr refId;
    format = "table";
    instant = true;
    range = false;
  };

  # qBittorrent tracker status enum → text/colour
  trackerStatusMap = [
    {
      type = "value";
      options = {
        "0" = {
          color = "text";
          index = 0;
          text = "Disabled";
        };
        "1" = {
          color = "yellow";
          index = 1;
          text = "Not contacted";
        };
        "2" = {
          color = "green";
          index = 2;
          text = "Working";
        };
        "3" = {
          color = "blue";
          index = 3;
          text = "Updating";
        };
        "4" = {
          color = "red";
          index = 4;
          text = "Not working";
        };
      };
    }
  ];

  # ── stat panel helper ──
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
      mappings ? [ ],
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
          inherit mappings unit;
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

  # ── timeseries panel helper ──
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
in
{
  folder = "Media";
  title = "qBittorrent";
  uid = "qbittorrent000001";
  description = "qBittorrent seedbox: transfer/peer/disk-queue/buffer health, VPN egress (exit IP + forwarded port + claimed tunnel via the vpn-gw broker), per-torrent + per-tracker tables. Full VPN metrics live on the VPN Gateway dashboard.";
  editable = true;
  fiscalYearStartMonth = 0;
  graphTooltip = 1;
  id = null;
  links = [ ];
  refresh = "30s";
  schemaVersion = 40;
  tags = [
    "media"
    "download"
    "torrent"
    "qbittorrent"
  ];
  timezone = "browser";
  version = 1;
  weekStart = "";

  time = {
    from = "now-6h";
    to = "now";
  };

  timepicker = { };

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

    # ── Row 1: overview stats ────────────────────────────────────────────
    (stat {
      id = 1;
      title = "Version";
      x = 0;
      y = 0;
      w = 3;
      expr = "qbittorrent_app_version{${host}}";
      legendFormat = "{{version}}";
      textMode = "name";
      colorMode = "none";
    })
    (stat {
      id = 2;
      title = "Torrents";
      x = 3;
      y = 0;
      w = 3;
      expr = "qbittorrent_global_torrents{${host}}";
      unit = "short";
    })
    (stat {
      id = 3;
      title = "Peer Connections";
      x = 6;
      y = 0;
      w = 3;
      expr = "qbittorrent_global_total_peer_connections{${host}}";
      unit = "short";
      graphMode = "area";
    })
    (stat {
      id = 4;
      title = "Reachability";
      x = 9;
      y = 0;
      w = 3;
      # value is always 1; the state is carried in the connection_status label
      expr = "qbittorrent_transfer_connection_status{${host}}";
      legendFormat = "{{connection_status}}";
      textMode = "name";
      colorMode = "none";
    })
    (stat {
      id = 5;
      title = "DL Speed";
      x = 12;
      y = 0;
      w = 3;
      expr = "qbittorrent_global_download_speed_bytes{${host}}";
      unit = "Bps";
      graphMode = "area";
    })
    (stat {
      id = 6;
      title = "UL Speed";
      x = 15;
      y = 0;
      w = 3;
      expr = "qbittorrent_global_upload_speed_bytes{${host}}";
      unit = "Bps";
      graphMode = "area";
    })
    (stat {
      id = 7;
      title = "Ratio";
      x = 18;
      y = 0;
      w = 3;
      expr = "qbittorrent_global_ratio{${host}}";
      unit = "none";
    })
    (stat {
      id = 8;
      title = "Free Disk";
      x = 21;
      y = 0;
      w = 3;
      expr = "qbittorrent_global_free_space_on_disk_bytes{${host}}";
      unit = "bytes";
    })

    # ── Row 2: VPN egress + geo + session ────────────────────────────────
    (stat {
      id = 10;
      title = "Exit IP";
      x = 0;
      y = 4;
      w = 4;
      # exit IP of qbt's currently-claimed tunnel: the broker's exit-IP info metric joined to
      # the claim on the `tunnel` label. Lives on vpn-gw (host="vpn-gw"), not qbt.
      expr = ''max by (exit_ip) (vpngw_tunnel_exit_ip * on(tunnel) group_left() vpngw_tunnel_claimed{owner="qbittorrent"})'';
      legendFormat = "{{exit_ip}}";
      textMode = "name";
      colorMode = "none";
    })
    (stat {
      id = 11;
      title = "VPN Tunnel";
      x = 4;
      y = 4;
      w = 5;
      # which pool tunnel qbt currently holds (geo/asn dropped with ipinfo → broker exports
      # exit IP only; re-add via a Vector geoip transform later if wanted).
      expr = ''vpngw_tunnel_claimed{owner="qbittorrent"}'';
      legendFormat = "{{tunnel}}";
      textMode = "name";
      colorMode = "none";
    })
    (stat {
      id = 12;
      title = "Tunnels Up";
      x = 9;
      y = 4;
      w = 3;
      # broker fleet health: how many pool tunnels are currently healthy.
      expr = "count(vpngw_tunnel_healthy == 1) or vector(0)";
      unit = "short";
    })
    (stat {
      id = 13;
      title = "Fwd Port";
      x = 12;
      y = 4;
      w = 2;
      expr = ''vpngw_tunnel_forwarded_port{owner="qbittorrent"}'';
      unit = "none";
    })
    (stat {
      id = 14;
      title = "Seeders";
      x = 14;
      y = 4;
      w = 2;
      expr = "sum(qbittorrent_torrent_seeders{${host}})";
      unit = "short";
    })
    (stat {
      id = 15;
      title = "Leechers";
      x = 16;
      y = 4;
      w = 2;
      expr = "sum(qbittorrent_torrent_leechers{${host}})";
      unit = "short";
    })
    (stat {
      id = 16;
      title = "Session Up";
      x = 18;
      y = 4;
      w = 3;
      expr = "qbittorrent_global_session_uploaded_bytes{${host}}";
      unit = "bytes";
    })
    (stat {
      id = 17;
      title = "Session Down";
      x = 21;
      y = 4;
      w = 3;
      expr = "qbittorrent_global_session_downloaded_bytes{${host}}";
      unit = "bytes";
    })

    # ── Row 3: global speed ──────────────────────────────────────────────
    (ts {
      id = 20;
      title = "Global Download / Upload Speed";
      x = 0;
      y = 8;
      w = 24;
      unit = "Bps";
      targets = [
        (target "qbittorrent_global_download_speed_bytes{${host}}" "Download" "A")
        (target "qbittorrent_global_upload_speed_bytes{${host}}" "Upload" "B")
      ];
    })

    # ── Row 4: tuning visibility — peers / buffers / disk queue ───────────
    (ts {
      id = 30;
      title = "Peer Connections";
      description = "Total established peer connections. uTP-off (TCP only) + connectionSpeed=300 drive how fast this fills; the swarm caps it.";
      x = 0;
      y = 16;
      w = 8;
      unit = "short";
      targets = [
        (target "qbittorrent_global_total_peer_connections{${host}}" "peers" "A")
      ];
    })
    (ts {
      id = 31;
      title = "libtorrent Buffers";
      description = "In-memory buffer bytes (send-queue that keeps high-BDP uploads fed off NFS, rises with sendBufferWatermark × active fast peers) + disk-write backlog waiting to flush to NFS.";
      x = 8;
      y = 16;
      w = 8;
      unit = "bytes";
      targets = [
        (target "qbittorrent_global_total_buffers_size_bytes{${host}}" "buffers" "A")
        (target "qbittorrent_global_total_queued_size_bytes{${host}}" "disk queue" "B")
      ];
    })
    {
      id = 32;
      type = "timeseries";
      title = "Disk I/O Queue";
      description = "Queued disk jobs + avg queue time. Sustained non-zero = qbt is disk-bound (NFS latency) — the read_ahead=16MB / asyncIO=128 / nconnect=16 tuning targets this.";
      pluginVersion = "11.4.0";
      gridPos = {
        h = 8;
        w = 8;
        x = 16;
        y = 16;
      };
      datasource = prom;
      fieldConfig = {
        defaults = tsDefaults // {
          unit = "short";
        };
        overrides = [
          {
            matcher = {
              id = "byName";
              options = "Avg Queue Time";
            };
            properties = [
              {
                id = "unit";
                value = "ms";
              }
              {
                id = "custom.axisPlacement";
                value = "right";
              }
            ];
          }
        ];
      };
      options = tsOptions;
      targets = [
        (target "qbittorrent_global_queued_io_jobs{${host}}" "Queued Jobs" "A")
        (target "qbittorrent_global_average_time_queue{${host}}" "Avg Queue Time" "B")
      ];
    }

    # ── Row 5: states + ratio ────────────────────────────────────────────
    {
      id = 40;
      type = "piechart";
      title = "Torrent States";
      pluginVersion = "11.4.0";
      gridPos = {
        h = 8;
        w = 8;
        x = 0;
        y = 24;
      };
      datasource = prom;
      fieldConfig = {
        defaults = {
          color.mode = "palette-classic";
          custom.hideFrom = {
            legend = false;
            tooltip = false;
            viz = false;
          };
          mappings = [ ];
          unit = "short";
        };
        overrides = [ ];
      };
      options = {
        legend = {
          displayMode = "table";
          placement = "right";
          showLegend = true;
          values = [ "value" ];
        };
        pieType = "pie";
        reduceOptions = {
          calcs = [ "lastNotNull" ];
          fields = "";
          values = false;
        };
        tooltip = {
          mode = "single";
          sort = "none";
        };
      };
      targets = [
        (targetInstant "qbittorrent_torrent_states{${host}} > 0" "{{name}}" "A")
      ];
    }
    (ts {
      id = 41;
      title = "Torrent Ratio";
      x = 8;
      y = 24;
      w = 16;
      unit = "none";
      targets = [
        (target "qbittorrent_torrent_ratio{${host}}" "{{name}}" "A")
      ];
    })

    # ── Row 6: per-torrent speeds ────────────────────────────────────────
    (ts {
      id = 50;
      title = "Download Speed by Torrent";
      x = 0;
      y = 32;
      w = 12;
      unit = "Bps";
      targets = [
        (target "qbittorrent_torrent_download_speed_bytes{${host}} > 0" "{{name}}" "A")
      ];
    })
    (ts {
      id = 51;
      title = "Upload Speed by Torrent";
      x = 12;
      y = 32;
      w = 12;
      unit = "Bps";
      targets = [
        (target "qbittorrent_torrent_upload_speed_bytes{${host}} > 0" "{{name}}" "A")
      ];
    })

    # ── Row 7: categories + tags ─────────────────────────────────────────
    {
      id = 60;
      type = "table";
      title = "Torrents by Category";
      pluginVersion = "11.4.0";
      gridPos = {
        h = 8;
        w = 12;
        x = 0;
        y = 40;
      };
      datasource = prom;
      fieldConfig = {
        defaults = {
          color.mode = "thresholds";
          custom = {
            align = "auto";
            cellOptions.type = "auto";
            inspect = false;
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
        overrides = [ ];
      };
      options = {
        cellHeight = "sm";
        footer = {
          countRows = false;
          fields = "";
          reducer = [ "sum" ];
          show = false;
        };
        showHeader = true;
      };
      targets = [
        (
          (targetInstant "count by (category) (max by (category, name) (qbittorrent_torrent_info{${host}}))"
            "__auto"
            "A"
          )
          // {
            format = "table";
          }
        )
      ];
      transformations = [
        {
          id = "organize";
          options = {
            excludeByName = {
              Time = true;
            };
            renameByName = {
              category = "Category";
              "Value" = "Torrents";
            };
          };
        }
      ];
    }
    {
      id = 61;
      type = "table";
      title = "Torrents by Tag";
      pluginVersion = "11.4.0";
      gridPos = {
        h = 8;
        w = 12;
        x = 12;
        y = 40;
      };
      datasource = prom;
      fieldConfig = {
        defaults = {
          color.mode = "thresholds";
          custom = {
            align = "auto";
            cellOptions.type = "auto";
            inspect = false;
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
        overrides = [ ];
      };
      options = {
        cellHeight = "sm";
        footer = {
          countRows = false;
          fields = "";
          reducer = [ "sum" ];
          show = false;
        };
        showHeader = true;
      };
      targets = [
        (
          (targetInstant "count by (tag) (qbittorrent_torrent_tags{${host}})" "__auto" "A")
          // {
            format = "table";
          }
        )
      ];
      transformations = [
        {
          id = "organize";
          options = {
            excludeByName = {
              Time = true;
            };
            renameByName = {
              tag = "Tag";
              "Value" = "Torrents";
            };
          };
        }
      ];
    }

    # ── Row 8: trackers (passkey stripped to host) ───────────────────────
    {
      id = 70;
      type = "table";
      title = "Trackers";
      description = "Per-tracker status + swarm counts. The announce URL's passkey is stripped to the host via label_replace so it isn't rendered.";
      pluginVersion = "11.4.0";
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 48;
      };
      datasource = prom;
      fieldConfig = {
        defaults = {
          color.mode = "thresholds";
          custom = {
            align = "auto";
            cellOptions.type = "auto";
            filterable = true;
            inspect = false;
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
        overrides = [
          {
            matcher = {
              id = "byName";
              options = "Status";
            };
            properties = [
              {
                id = "mappings";
                value = trackerStatusMap;
              }
              {
                id = "custom.cellOptions";
                value = {
                  type = "color-text";
                };
              }
            ];
          }
        ];
      };
      options = {
        cellHeight = "sm";
        footer = {
          countRows = false;
          fields = "";
          reducer = [ "sum" ];
          show = false;
        };
        showHeader = true;
        sortBy = [ { displayName = "Tracker"; } ];
      };
      targets = [
        (tableTarget ''label_replace(qbittorrent_tracker_status{${host}}, "tracker", "$1", "url", "https?://([^/]+)/.*")'' "A")
        (tableTarget ''label_replace(qbittorrent_tracker_seeders{${host}}, "tracker", "$1", "url", "https?://([^/]+)/.*")'' "B")
        (tableTarget ''label_replace(qbittorrent_tracker_leeches{${host}}, "tracker", "$1", "url", "https?://([^/]+)/.*")'' "C")
        (tableTarget ''label_replace(qbittorrent_tracker_peers{${host}}, "tracker", "$1", "url", "https?://([^/]+)/.*")'' "D")
        (tableTarget ''label_replace(qbittorrent_tracker_downloaded{${host}}, "tracker", "$1", "url", "https?://([^/]+)/.*")'' "E")
      ];
      transformations = [
        {
          id = "joinByField";
          options = {
            byField = "tracker";
            mode = "outer";
          };
        }
        {
          id = "filterFieldsByName";
          options.include.pattern = "^(tracker|Value #[A-E])$";
        }
        {
          id = "organize";
          options = {
            excludeByName = { };
            indexByName = {
              tracker = 0;
              "Value #A" = 1;
              "Value #B" = 2;
              "Value #C" = 3;
              "Value #D" = 4;
              "Value #E" = 5;
            };
            renameByName = {
              tracker = "Tracker";
              "Value #A" = "Status";
              "Value #B" = "Seeders";
              "Value #C" = "Leeches";
              "Value #D" = "Peers";
              "Value #E" = "Downloaded";
            };
          };
        }
      ];
    }

    # ── Row 9: per-torrent table (high cardinality) ──────────────────────
    {
      id = 80;
      type = "table";
      title = "Torrents";
      pluginVersion = "11.4.0";
      gridPos = {
        h = 10;
        w = 24;
        x = 0;
        y = 56;
      };
      datasource = prom;
      fieldConfig = {
        defaults = {
          color.mode = "thresholds";
          custom = {
            align = "auto";
            cellOptions.type = "auto";
            filterable = true;
            inspect = false;
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
        overrides = [
          {
            matcher = {
              id = "byName";
              options = "Size";
            };
            properties = [
              {
                id = "unit";
                value = "bytes";
              }
            ];
          }
          {
            matcher = {
              id = "byName";
              options = "DL";
            };
            properties = [
              {
                id = "unit";
                value = "Bps";
              }
            ];
          }
          {
            matcher = {
              id = "byName";
              options = "UL";
            };
            properties = [
              {
                id = "unit";
                value = "Bps";
              }
            ];
          }
          {
            matcher = {
              id = "byName";
              options = "Progress";
            };
            properties = [
              {
                id = "unit";
                value = "percentunit";
              }
            ];
          }
          {
            matcher = {
              id = "byName";
              options = "ETA";
            };
            properties = [
              {
                id = "unit";
                value = "s";
              }
            ];
          }
        ];
      };
      options = {
        cellHeight = "sm";
        footer = {
          countRows = false;
          fields = "";
          reducer = [ "sum" ];
          show = false;
        };
        showHeader = true;
        sortBy = [
          {
            desc = true;
            displayName = "UL";
          }
        ];
      };
      # Numeric columns come from the dedicated per-torrent metrics (keyed on `name`
      # only, so their value churns without minting new series). qbittorrent_torrent_info
      # is used ONLY (name-deduped) for the stable Category/Tags columns — with
      # ENABLE_HIGH_CARDINALITY it encodes changing values as labels, so using it for
      # numerics would return every stale series in the staleness window → duplicate rows.
      targets = [
        (
          (targetInstant "max by (name, category, tags) (qbittorrent_torrent_info{${host}})" "__auto" "A")
          // {
            format = "table";
          }
        )
        (
          (targetInstant "max by (name) (qbittorrent_torrent_progress{${host}})" "__auto" "B")
          // {
            format = "table";
          }
        )
        (
          (targetInstant "max by (name) (qbittorrent_torrent_ratio{${host}})" "__auto" "C")
          // {
            format = "table";
          }
        )
        (
          (targetInstant "max by (name) (qbittorrent_torrent_seeders{${host}})" "__auto" "D")
          // {
            format = "table";
          }
        )
        (
          (targetInstant "max by (name) (qbittorrent_torrent_leechers{${host}})" "__auto" "E")
          // {
            format = "table";
          }
        )
        (
          (targetInstant "max by (name) (qbittorrent_torrent_download_speed_bytes{${host}})" "__auto" "F")
          // {
            format = "table";
          }
        )
        (
          (targetInstant "max by (name) (qbittorrent_torrent_upload_speed_bytes{${host}})" "__auto" "G")
          // {
            format = "table";
          }
        )
        (
          (targetInstant "max by (name) (qbittorrent_torrent_eta{${host}})" "__auto" "H")
          // {
            format = "table";
          }
        )
        (
          (targetInstant "max by (name) (qbittorrent_torrent_size_bytes{${host}})" "__auto" "I")
          // {
            format = "table";
          }
        )
      ];
      transformations = [
        {
          id = "joinByField";
          options = {
            byField = "name";
            mode = "outer";
          };
        }
        {
          id = "filterFieldsByName";
          options.include.pattern = "^(name|category|tags|Value #[B-I])$";
        }
        {
          id = "organize";
          options = {
            excludeByName = { };
            indexByName = {
              name = 0;
              category = 1;
              tags = 2;
              "Value #B" = 3;
              "Value #C" = 4;
              "Value #D" = 5;
              "Value #E" = 6;
              "Value #F" = 7;
              "Value #G" = 8;
              "Value #H" = 9;
              "Value #I" = 10;
            };
            renameByName = {
              name = "Name";
              category = "Category";
              tags = "Tags";
              "Value #B" = "Progress";
              "Value #C" = "Ratio";
              "Value #D" = "Seeders";
              "Value #E" = "Leechers";
              "Value #F" = "DL";
              "Value #G" = "UL";
              "Value #H" = "ETA";
              "Value #I" = "Size";
            };
          };
        }
      ];
    }

    # ── Row 10: efficiency & limits (stats) ──────────────────────────────
    (stat {
      id = 90;
      title = "All-time Up";
      x = 0;
      y = 66;
      w = 3;
      expr = "qbittorrent_global_alltime_uploaded_bytes{${host}}";
      unit = "bytes";
    })
    (stat {
      id = 91;
      title = "All-time Down";
      x = 3;
      y = 66;
      w = 3;
      expr = "qbittorrent_global_alltime_downloaded_bytes{${host}}";
      unit = "bytes";
    })
    (stat {
      id = 92;
      title = "All-time Ratio";
      x = 6;
      y = 66;
      w = 3;
      expr = "qbittorrent_global_alltime_uploaded_bytes{${host}} / clamp_min(qbittorrent_global_alltime_downloaded_bytes{${host}}, 1)";
      unit = "none";
    })
    (stat {
      id = 93;
      title = "Session Ratio";
      x = 9;
      y = 66;
      w = 3;
      expr = "qbittorrent_global_session_uploaded_bytes{${host}} / clamp_min(qbittorrent_global_session_downloaded_bytes{${host}}, 1)";
      unit = "none";
    })
    (stat {
      id = 94;
      title = "UL Limit";
      x = 12;
      y = 66;
      w = 3;
      expr = "qbittorrent_global_upload_rate_limit_bytes{${host}}";
      unit = "Bps";
      mappings = [
        {
          type = "value";
          options."0" = {
            text = "Unlimited";
            index = 0;
          };
        }
      ];
    })
    (stat {
      id = 95;
      title = "DL Limit";
      x = 15;
      y = 66;
      w = 3;
      expr = "qbittorrent_global_download_rate_limit_bytes{${host}}";
      unit = "Bps";
      mappings = [
        {
          type = "value";
          options."0" = {
            text = "Unlimited";
            index = 0;
          };
        }
      ];
    })
    (stat {
      id = 96;
      title = "Alt-Speed";
      x = 18;
      y = 66;
      w = 3;
      expr = "qbittorrent_app_alt_rate_limits_enabled{${host}}";
      textMode = "value";
      colorMode = "value";
      mappings = [
        {
          type = "value";
          options = {
            "0" = {
              text = "Off";
              color = "green";
              index = 0;
            };
            "1" = {
              text = "On";
              color = "yellow";
              index = 1;
            };
          };
        }
      ];
    })
    (stat {
      id = 97;
      title = "Wasted (session)";
      x = 21;
      y = 66;
      w = 3;
      expr = "qbittorrent_global_total_wasted_session_bytes{${host}}";
      unit = "bytes";
    })

    # ── Row 11: efficiency timeseries ────────────────────────────────────
    (ts {
      id = 100;
      title = "Wasted Bandwidth";
      description = "rate(total_wasted_session_bytes). Bytes downloaded then discarded — hashfail, redundant/endgame pieces. A sustained spike is a bad peer or storage/hash trouble; it burns the same NFS + net as seeding.";
      x = 0;
      y = 70;
      w = 8;
      unit = "Bps";
      targets = [
        (target "rate(qbittorrent_global_total_wasted_session_bytes{${host}}[5m])" "wasted" "A")
      ];
    })
    (ts {
      id = 101;
      title = "Avg Upload per Peer";
      description = "upload_speed / peer_connections. Mean throughput each connected peer receives; a low value at high peer count points at send-buffer/slot starvation rather than lack of demand.";
      x = 8;
      y = 70;
      w = 8;
      unit = "Bps";
      targets = [
        (target
          "qbittorrent_global_upload_speed_bytes{${host}} / clamp_min(qbittorrent_global_total_peer_connections{${host}}, 1)"
          "per-peer"
          "A"
        )
      ];
    })
    (ts {
      id = 102;
      title = "Active Seeding vs Leeching";
      description = "Count of torrents with live up/down speed. max_active_* is inert here (QueueingSystemEnabled=false) — this is the real concurrency.";
      x = 16;
      y = 70;
      w = 8;
      unit = "short";
      targets = [
        (target "count(qbittorrent_torrent_upload_speed_bytes{${host}} > 0)" "seeding" "A")
        (target "count(qbittorrent_torrent_download_speed_bytes{${host}} > 0)" "leeching" "B")
      ];
    })

    # ── Row 12: peer choke/interest (exporter.py flag aggregation) ───────
    (ts {
      id = 110;
      title = "Peer Upload State";
      description = "Per-peer flags aggregated across active torrents. uploading (U) = slots in use; interested-choked (u) = peers who want data while we choke them — with maxUploads=-1 this stays 0, and any sustained climb is proof a slot limit would throttle upload.";
      x = 0;
      y = 78;
      w = 12;
      unit = "short";
      targets = [
        (target "qbt_peers_uploading{${host}}" "uploading (U)" "A")
        (target "qbt_peers_interested_choked{${host}}" "interested-choked (u)" "B")
        (target "qbt_peers_downloading{${host}}" "downloading (D)" "C")
        (target "qbt_peers_optimistic{${host}}" "optimistic (O)" "D")
        (target "qbt_peers_snubbed{${host}}" "snubbed (S)" "E")
      ];
    })
    (ts {
      id = 111;
      title = "Peer Connection Type";
      description = "Transport mix. btProtocol=TCP → all TCP/BT; uTP appears only if the protocol is switched to Both.";
      x = 12;
      y = 78;
      w = 6;
      unit = "short";
      targets = [
        (target "qbt_peers_conn_bt{${host}}" "TCP/BT" "A")
        (target "qbt_peers_conn_utp{${host}}" "uTP" "B")
        (target "qbt_peers_conn_web{${host}}" "Web" "C")
      ];
    })
    (ts {
      id = 112;
      title = "Peer Scrape Coverage";
      description = "scraped < active (or qbt_scrape_capped=1) means the top-upspeed cap sampled the busiest torrents rather than all active — the flag counts above are then a sample, not a total.";
      x = 18;
      y = 78;
      w = 6;
      unit = "short";
      targets = [
        (target "qbt_scrape_torrents_active{${host}}" "active" "A")
        (target "qbt_scrape_torrents_scraped{${host}}" "scraped" "B")
        (target "qbt_scrape_capped{${host}}" "capped" "C")
      ];
    })

    # ── Row 13: swarm demand & reach (exporter.py + martabal) ───────────
    (ts {
      id = 120;
      title = "Swarm Demand vs Reach";
      description = "The seeding funnel. addressable = leechers across every swarm we're in (sum of per-torrent leechers, martabal). connected = all peer connections qbt holds. uploading = peers actually receiving. A wide addressable→connected gap is unreached demand (inbound reachability); a wide connected→uploading gap is peers that already have what we'd serve.";
      x = 0;
      y = 86;
      w = 12;
      unit = "short";
      targets = [
        (target "sum(qbittorrent_torrent_leechers{${host}})" "addressable leechers" "A")
        (target "qbittorrent_global_total_peer_connections{${host}}" "connected peers" "B")
        (target "qbt_peers_upload_active{${host}}" "uploading" "C")
      ];
    })
    (ts {
      id = 121;
      title = "Peer Direction";
      description = "incoming (I) = peers that dialed us; outgoing = peers we dialed. A low incoming share means leechers can't reach us — check the VPN forwarded port matches the listen port.";
      x = 12;
      y = 86;
      w = 6;
      unit = "short";
      targets = [
        (target "qbt_peers_incoming{${host}}" "incoming (I)" "A")
        (target "qbt_peers_outgoing{${host}}" "outgoing" "B")
      ];
    })
    (ts {
      id = 122;
      title = "Fastest Peer Upload";
      description = "Top single-peer upload rate across active torrents. If this presses toward ~25 MB/s the per-peer send-buffer watermark is the ceiling and is worth raising; today's peers sit far below it.";
      x = 18;
      y = 86;
      w = 6;
      unit = "Bps";
      targets = [
        (target "qbt_peer_up_max{${host}}" "fastest peer" "A")
      ];
    })

    # ── Row 14: upload split by transport (uTP-trial verdict) ───────────
    (ts {
      id = 123;
      title = "Upload by Transport";
      description = "Upload rate summed per peer transport across active torrents. With btProtocol=Both this is the uTP trial's verdict: uTP (LEDBAT) carrying real bytes = reach gain worth keeping; uTP flat near 0 while TCP holds = uTP peers aren't there or LEDBAT throttled them to nothing, revert to TCP-only.";
      x = 0;
      y = 94;
      w = 12;
      unit = "Bps";
      targets = [
        (target "qbt_up_speed_tcp{${host}}" "TCP" "A")
        (target "qbt_up_speed_utp{${host}}" "uTP" "B")
        (target "qbt_up_speed_web{${host}}" "Web" "C")
      ];
    })

  ];
}
