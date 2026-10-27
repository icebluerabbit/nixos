let
  prom = {
    type = "prometheus";
    uid = "Prometheus";
  };

  host = ''host="vpn-gw"'';

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
  folder = "Network";
  title = "VPN Gateway (ProtonVPN / WireGuard)";
  uid = "vpngw8f3a2b1c9d4e5";
  description = "10-tunnel ProtonVPN WireGuard broker: per-tunnel handshake/throughput (wireguard-exporter) + exit-IP/health/claim/forwarded-port (broker /export) + host NAT/forwarding/net (node-exporter). vpn-gw's broker owns tunnel selection, health, and NAT-PMP leasing; clients claim a tunnel and bind it.";
  editable = true;
  fiscalYearStartMonth = 0;
  graphTooltip = 1;
  id = null;
  links = [ ];
  refresh = "30s";
  schemaVersion = 40;
  tags = [
    "network"
    "vpn"
    "wireguard"
    "proton"
  ];
  timezone = "browser";
  version = 1;
  weekStart = "";

  time = {
    from = "now-3h";
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

    # ── Row 1: headline stats ────────────────────────────────────────────
    (stat {
      id = 1;
      title = "Tunnels Up";
      x = 0;
      y = 0;
      w = 4;
      # a tunnel is "up" when its last handshake is fresh (< 3 min)
      expr = "count(time() - wireguard_latest_handshake_seconds{${host}} < 180) or vector(0)";
      unit = "short";
      colorMode = "background";
      thresholds = [
        {
          color = "red";
          value = null;
        }
        {
          color = "yellow";
          value = 1;
        }
        {
          color = "green";
          value = 10;
        }
      ];
    })
    (stat {
      id = 2;
      title = "Tunnel RX";
      x = 4;
      y = 0;
      w = 4;
      expr = "sum(rate(wireguard_received_bytes_total{${host}}[$__rate_interval]))";
      unit = "Bps";
      graphMode = "area";
    })
    (stat {
      id = 3;
      title = "Tunnel TX";
      x = 8;
      y = 0;
      w = 4;
      expr = "sum(rate(wireguard_sent_bytes_total{${host}}[$__rate_interval]))";
      unit = "Bps";
      graphMode = "area";
    })
    (stat {
      id = 4;
      title = "NAT Conntrack";
      x = 12;
      y = 0;
      w = 4;
      expr = "node_nf_conntrack_entries{${host}}";
      unit = "short";
      graphMode = "area";
    })
    (stat {
      id = 5;
      title = "CPU";
      x = 16;
      y = 0;
      w = 4;
      expr = ''100 * (1 - avg(rate(node_cpu_seconds_total{${host},mode="idle"}[$__rate_interval])))'';
      unit = "percent";
      graphMode = "area";
      colorMode = "background";
      thresholds = [
        {
          color = "green";
          value = null;
        }
        {
          color = "yellow";
          value = 70;
        }
        {
          color = "red";
          value = 90;
        }
      ];
    })
    (stat {
      id = 6;
      title = "Load1";
      x = 20;
      y = 0;
      w = 4;
      expr = "node_load1{${host}}";
      unit = "short";
      graphMode = "area";
    })

    # ── Row 2: per-tunnel table (handshake + throughput, joined on interface) ──
    {
      id = 10;
      type = "table";
      title = "Tunnels";
      pluginVersion = "11.4.0";
      gridPos = {
        h = 15;
        w = 24;
        x = 0;
        y = 4;
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
              options = "Handshake Age";
            };
            properties = [
              {
                id = "unit";
                value = "s";
              }
              {
                id = "custom.cellOptions";
                value = {
                  type = "color-text";
                };
              }
              {
                id = "thresholds";
                value = {
                  mode = "absolute";
                  steps = [
                    {
                      color = "green";
                      value = null;
                    }
                    {
                      color = "yellow";
                      value = 150;
                    }
                    {
                      color = "red";
                      value = 180;
                    }
                  ];
                };
              }
            ];
          }
          {
            matcher = {
              id = "byRegexp";
              options = "^(RX|TX)$";
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
              id = "byRegexp";
              options = "^Total (RX|TX)$";
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
              options = "Health";
            };
            properties = [
              {
                id = "custom.cellOptions";
                value.type = "color-background";
              }
              {
                id = "thresholds";
                value = {
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
              }
              {
                id = "mappings";
                value = [
                  {
                    type = "value";
                    options = {
                      "0".text = "Down";
                      "1".text = "Up";
                    };
                  }
                ];
              }
            ];
          }
          {
            matcher = {
              id = "byName";
              options = "Fwd Port";
            };
            properties = [
              {
                id = "unit";
                value = "none";
              }
            ];
          }
          {
            matcher = {
              id = "byName";
              options = "Flaps 24h";
            };
            properties = [
              {
                id = "unit";
                value = "none";
              }
              {
                id = "custom.cellOptions";
                value.type = "color-text";
              }
              {
                id = "thresholds";
                value = {
                  mode = "absolute";
                  steps = [
                    {
                      color = "green";
                      value = null;
                    }
                    {
                      color = "yellow";
                      value = 1;
                    }
                    {
                      color = "red";
                      value = 5;
                    }
                  ];
                };
              }
            ];
          }
          {
            matcher = {
              id = "byName";
              options = "Latency";
            };
            properties = [
              {
                id = "unit";
                value = "s";
              }
              {
                id = "custom.cellOptions";
                value.type = "color-text";
              }
              {
                id = "thresholds";
                value = {
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
                      value = 0.25;
                    }
                  ];
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
        sortBy = [ { displayName = "Tunnel"; } ];
      };
      targets = [
        (tableTarget "time() - wireguard_latest_handshake_seconds{${host}}" "A")
        (tableTarget "rate(wireguard_received_bytes_total{${host}}[$__rate_interval])" "B")
        (tableTarget "rate(wireguard_sent_bytes_total{${host}}[$__rate_interval])" "C")
        (tableTarget "wireguard_received_bytes_total{${host}}" "D")
        (tableTarget "wireguard_sent_bytes_total{${host}}" "E")
        # broker state, joined on interface (label_replace tunnel → interface).
        (tableTarget ''label_replace(vpngw_tunnel_exit_ip{${host}}, "interface", "$1", "tunnel", "(.*)")'' "F")
        (tableTarget ''label_replace(vpngw_tunnel_healthy{${host}}, "interface", "$1", "tunnel", "(.*)")'' "G")
        # strip owner here (max without) so the ONLY owner-bearing frame is claimed (#I); else the
        # join yields two `owner` fields and the owner→Owner rename can't target either cleanly.
        (tableTarget ''label_replace(max without (owner) (vpngw_tunnel_forwarded_port{${host}}), "interface", "$1", "tunnel", "(.*)")'' "H")
        (tableTarget ''label_replace(vpngw_tunnel_claimed{${host}}, "interface", "$1", "tunnel", "(.*)")'' "I")
        (tableTarget ''label_replace(vpngw_tunnel_flap_score{${host}}, "interface", "$1", "tunnel", "(.*)")'' "J")
        (tableTarget ''label_replace(vpngw_tunnel_latency_seconds{${host}}, "interface", "$1", "tunnel", "(.*)")'' "K")
      ];
      transformations = [
        {
          id = "joinByField";
          options = {
            byField = "interface";
            mode = "outer";
          };
        }
        {
          id = "filterFieldsByName";
          # keep the wireguard values (#A-E), the broker health/port/flap/latency values
          # (#G/#H/#J/#K), and the exit_ip + owner LABELS (drop #F/#I values — labels only).
          options.include.pattern = "^(interface|exit_ip|owner|Value #[A-E]|Value #[GHJK])$";
        }
        {
          id = "organize";
          options = {
            excludeByName = { };
            indexByName = {
              interface = 0;
              owner = 1;
              "Value #G" = 2;
              "Value #H" = 3;
              "Value #J" = 4;
              "Value #K" = 5;
              exit_ip = 6;
              "Value #A" = 7;
              "Value #B" = 8;
              "Value #C" = 9;
              "Value #D" = 10;
              "Value #E" = 11;
            };
            renameByName = {
              interface = "Tunnel";
              owner = "Owner";
              "Value #G" = "Health";
              "Value #H" = "Fwd Port";
              "Value #J" = "Flaps 24h";
              "Value #K" = "Latency";
              exit_ip = "Exit IP";
              "Value #A" = "Handshake Age";
              "Value #B" = "RX";
              "Value #C" = "TX";
              "Value #D" = "Total RX";
              "Value #E" = "Total TX";
            };
          };
        }
      ];
    }

    # ── Row 3: per-tunnel throughput (wireguard exporter) ────────────────
    (ts {
      id = 20;
      title = "Per-Tunnel RX (from Proton)";
      description = "Decrypted inbound per wg tunnel. This is qbt's download egressing the VPN.";
      x = 0;
      y = 19;
      w = 12;
      unit = "Bps";
      targets = [
        (target "rate(wireguard_received_bytes_total{${host}}[$__rate_interval])" "{{interface}}" "A")
      ];
    })
    (ts {
      id = 21;
      title = "Per-Tunnel TX (to Proton)";
      description = "Encrypted outbound per wg tunnel. This is qbt's upload/seeding.";
      x = 12;
      y = 19;
      w = 12;
      unit = "Bps";
      targets = [
        (target "rate(wireguard_sent_bytes_total{${host}}[$__rate_interval])" "{{interface}}" "A")
      ];
    })

    # ── Row 4: tunnel liveness ───────────────────────────────────────────
    (ts {
      id = 30;
      title = "Handshake Age";
      description = "Seconds since last WireGuard handshake per tunnel; a healthy tunnel re-handshakes ~every 2min (sawtooth). Flat climb past ~180s = the tunnel is dead/stalled.";
      x = 0;
      y = 27;
      w = 12;
      unit = "s";
      targets = [
        (target "time() - wireguard_latest_handshake_seconds{${host}}" "{{interface}}" "A")
      ];
    })
    (ts {
      id = 31;
      title = "Per-Tunnel Cumulative Transfer";
      description = "Total bytes moved per tunnel since bring-up (counter). Shows which tunnel is actually carrying qbt.";
      x = 12;
      y = 27;
      w = 12;
      unit = "bytes";
      targets = [
        (target "wireguard_received_bytes_total{${host}}" "{{interface}} RX" "A")
        (target "wireguard_sent_bytes_total{${host}}" "{{interface}} TX" "B")
      ];
    })

    # ── Row 5: forwarding path (host node-exporter) ──────────────────────
    (ts {
      id = 40;
      title = "Gateway eth0 (uplink to wg endpoints)";
      description = "eth0 = vpn-gw's LAN NIC. Carries the ENCRYPTED wg packets to/from Proton + vpn-gw's own traffic. Should mirror total tunnel TX/RX.";
      x = 0;
      y = 35;
      w = 12;
      unit = "Bps";
      targets = [
        (target ''rate(node_network_receive_bytes_total{${host},device="eth0"}[$__rate_interval])''
          "eth0 RX"
          "A"
        )
        (target ''rate(node_network_transmit_bytes_total{${host},device="eth0"}[$__rate_interval])''
          "eth0 TX"
          "B"
        )
      ];
    })
    (ts {
      id = 41;
      title = "Client Veths (gw-vpnN → qbt)";
      description = "Per-tunnel p2p veth toward the client. RX here = traffic qbt sent to route out the tunnel; TX = traffic returning to qbt.";
      x = 12;
      y = 35;
      w = 12;
      unit = "Bps";
      targets = [
        (target ''rate(node_network_receive_bytes_total{${host},device=~"gw-vpn.*"}[$__rate_interval])''
          "{{device}} RX"
          "A"
        )
        (target ''rate(node_network_transmit_bytes_total{${host},device=~"gw-vpn.*"}[$__rate_interval])''
          "{{device}} TX"
          "B"
        )
      ];
    })

    # ── Row 6: NAT + host resources ──────────────────────────────────────
    (ts {
      id = 50;
      title = "NAT Connection Tracking";
      description = "Conntrack entries vs limit. vpn-gw masquerades all client egress + DNATs inbound → every torrent peer = an entry. Nearing the limit drops connections.";
      x = 0;
      y = 43;
      w = 12;
      unit = "short";
      targets = [
        (target "node_nf_conntrack_entries{${host}}" "Entries" "A")
        (target "node_nf_conntrack_entries_limit{${host}}" "Limit" "B")
      ];
    })
    (ts {
      id = 51;
      title = "Host CPU / Memory";
      description = "wg encryption is CPU-bound; watch for a core pegging (softirq) capping throughput.";
      x = 12;
      y = 43;
      w = 12;
      unit = "percent";
      targets = [
        (target ''100 * (1 - avg(rate(node_cpu_seconds_total{${host},mode="idle"}[$__rate_interval])))''
          "CPU %"
          "A"
        )
        (target "100 * (1 - node_memory_MemAvailable_bytes{${host}} / node_memory_MemTotal_bytes{${host}})"
          "Mem %"
          "B"
        )
      ];
    })

    # ── Row 7: drops / errors ────────────────────────────────────────────
    (ts {
      id = 60;
      title = "Packet Drops / Errors (eth0 + wg* + gw-vpn*)";
      description = "Non-zero = the path is lossy → uTP/LEDBAT collapse, retransmits, throughput caps. eth0 RX drops under load = ring-buffer/softirq pressure.";
      x = 0;
      y = 51;
      w = 24;
      unit = "pps";
      targets = [
        (target
          ''rate(node_network_receive_drop_total{${host},device=~"eth0|wg[0-9]+|gw-vpn.*"}[$__rate_interval])''
          "{{device}} RX drop"
          "A"
        )
        (target
          ''rate(node_network_transmit_drop_total{${host},device=~"eth0|wg[0-9]+|gw-vpn.*"}[$__rate_interval])''
          "{{device}} TX drop"
          "B"
        )
        (target
          ''rate(node_network_receive_errs_total{${host},device=~"eth0|wg[0-9]+|gw-vpn.*"}[$__rate_interval])''
          "{{device}} RX err"
          "C"
        )
      ];
    })

    # ── Row 8: flap score + tunnel latency (broker /export) ───────────────
    (ts {
      id = 70;
      title = "Flap Score (24h rolling)";
      description = "Down-transitions per tunnel in the last 24h (broker, in-memory — resets on broker/container restart). Soft-deprioritizes flappy tunnels at claim time; a steadily-climbing line = a chronically unstable server worth regenerating.";
      x = 0;
      y = 59;
      w = 12;
      unit = "short";
      targets = [
        (target "vpngw_tunnel_flap_score{${host}}" "{{tunnel}}" "A")
      ];
    })
    (ts {
      id = 71;
      title = "Tunnel Latency (ICMP RTT via tunnel)";
      description = "Per-tunnel RTT: ICMP to 1.1.1.1 bound to each wg iface, probed every 30s. Dominated by the path to the Proton exit city. Selection tie-breaker after flap score — among equally-stable tunnels the lowest-RTT one is claimed.";
      x = 12;
      y = 59;
      w = 12;
      unit = "s";
      targets = [
        (target "vpngw_tunnel_latency_seconds{${host}}" "{{tunnel}}" "A")
      ];
    })

    # ── Row 9: route-flap damping / tri-state (RFC 2439) ──────────────────
    (ts {
      id = 72;
      title = "Pool State (failing / connected / stable)";
      description = "Tunnel count per tri-state. `failing` = down or damping-suppressed (never selected); `connected` = up but young or still carrying penalty; `stable` = up past stableAfterSeconds with the penalty decayed under dampReuse. Only `stable` is allowed to consume a MAM dynamic-seedbox IP change. Watch for `stable` collapsing toward 0 — that is the shape of a provider-wide outage (2026-08-12: 4 of 10 down for an hour, discovered only via the tracker's reject log).";
      x = 0;
      y = 67;
      w = 12;
      unit = "short";
      targets = [
        (target "sum by (state) (vpngw_tunnel_state{${host}})" "{{state}}" "A")
      ];
    })
    (ts {
      id = 73;
      title = "Damping Penalty (RFC 2439)";
      description = "Per-tunnel flap-damping penalty. Each outage episode charges dampPenalty (1000) and decays with a 15min half-life. Above dampSuppress (2000 — two outages inside a half-life) the tunnel is skipped by selection until decay carries it under dampReuse (750); the gap between the two is the hysteresis that stops an oscillating tunnel being re-picked on every upswing. A line parked above 2000 = a server worth regenerating.";
      x = 12;
      y = 67;
      w = 12;
      unit = "short";
      targets = [
        (target "vpngw_tunnel_penalty{${host}}" "{{tunnel}}" "A")
      ];
    })
    (ts {
      id = 74;
      title = "Tunnel Dwell (continuous uptime)";
      description = "Time since each tunnel's last up-transition; resets to 0 on every outage episode. This is the clock behind `stable` — a tunnel must hold stableAfterSeconds (5min) before a client will commit an expensive side effect to its exit IP. Absent series = tunnel currently down.";
      x = 0;
      y = 75;
      w = 24;
      unit = "s";
      targets = [
        (target "vpngw_tunnel_uptime_seconds{${host}}" "{{tunnel}}" "A")
      ];
    })
  ];
}
