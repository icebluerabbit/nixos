let
  prom = {
    type = "prometheus";
    uid = "Prometheus";
  };
  loki = {
    type = "loki";
    uid = "Loki";
  };

  ev = e: ''soularr_events_total{event="${e}"}'';

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
      graphMode ? "none",
      description ? "",
    }:
    {
      inherit id title description;
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
        inherit graphMode;
        colorMode = "value";
        justifyMode = "center";
        orientation = "auto";
        reduceOptions = {
          calcs = [ "lastNotNull" ];
          fields = "";
          values = false;
        };
        textMode = "auto";
        wideLayout = true;
      };
      targets = [ (targetInstant expr "__auto" "A") ];
    };

  ts =
    {
      id,
      title,
      x,
      y,
      w,
      h ? 8,
      unit ? "none",
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
  title = "Soularr";
  uid = "soularr5f2a8c1d3e40";
  description = "soularr (Lidarr wanted list → slskd) automation. No /metrics endpoint — all figures are derived from the container journal via the Vector sidecar (log_to_metric): per-run grab counts + lifetime event counters + logs (Loki).";
  editable = true;
  graphTooltip = 1;
  refresh = "30s";
  schemaVersion = 40;
  tags = [
    "media"
    "soulseek"
    "soularr"
    "automation"
  ];
  timezone = "browser";
  time = {
    from = "now-24h";
    to = "now";
  };
  templating.list = [ ];
  annotations.list = [ ];

  panels = [
    # ── Row 1: headline counters ─────────────────────────────────────────
    (stat {
      id = 1;
      title = "Downloads Added (last run)";
      description = "Albums enqueued to slskd on the most recent run (Total Downloads added).";
      x = 0;
      y = 0;
      w = 4;
      expr = "soularr_downloads_added";
      graphMode = "area";
    })
    (stat {
      id = 2;
      title = "Failed Grabs (last run)";
      description = "Wanted releases that found no acceptable match on the last run.";
      x = 4;
      y = 0;
      w = 4;
      expr = "soularr_failed_grab";
    })
    (stat {
      id = 3;
      title = "Completed (total)";
      x = 8;
      y = 0;
      w = 4;
      expr = "sum(${ev "download_completed"})";
    })
    (stat {
      id = 4;
      title = "Matches (total)";
      x = 12;
      y = 0;
      w = 4;
      expr = "sum(${ev "match"})";
    })
    (stat {
      id = 5;
      title = "Import Attempts (total)";
      x = 16;
      y = 0;
      w = 4;
      expr = "sum(${ev "import_attempt"})";
    })
    (stat {
      id = 6;
      title = "Denylisted (total)";
      description = "Releases parked after a failed import so they aren't retried forever.";
      x = 20;
      y = 0;
      w = 4;
      expr = "sum(${ev "denylist"})";
    })

    # ── Row 2: activity + failures ───────────────────────────────────────
    (ts {
      id = 10;
      title = "Activity";
      description = "Searches issued, matches found, and downloads completed (per-second rates).";
      x = 0;
      y = 4;
      w = 12;
      unit = "cps";
      targets = [
        (target "sum(rate(${ev "search"}[$__rate_interval]))" "searches" "A")
        (target "sum(rate(${ev "match"}[$__rate_interval]))" "matches" "B")
        (target "sum(rate(${ev "download_completed"}[$__rate_interval]))" "completed" "C")
      ];
    })
    (ts {
      id = 11;
      title = "Failures";
      description = "Search failures, download failures, and denylist additions (per-second rates).";
      x = 12;
      y = 4;
      w = 12;
      unit = "cps";
      targets = [
        (target "sum(rate(${ev "search_failed"}[$__rate_interval]))" "search failed" "A")
        (target "sum(rate(${ev "download_failed"}[$__rate_interval]))" "download failed" "B")
        (target "sum(rate(${ev "denylist"}[$__rate_interval]))" "denylisted" "C")
      ];
    })

    # ── Row 3: logs ──────────────────────────────────────────────────────
    {
      id = 20;
      type = "logs";
      title = "soularr Logs";
      description = "Container journal via the Vector sidecar → Loki.";
      pluginVersion = "11.4.0";
      gridPos = {
        h = 8;
        w = 24;
        x = 0;
        y = 12;
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
          expr = ''{host="soularr"}'';
          queryType = "range";
          refId = "A";
        }
      ];
    }
  ];
}
