/*
  Shared panel constructors for the provisioned dashboards.

  Every dashboard here is a Nix attrset serialised straight to a Grafana
  dashboard JSON, which means the panel boilerplate (fieldConfig defaults,
  legend options, reduceOptions, gridPos) is identical in every file and
  tediously easy to get subtly wrong per-panel. This collects it once.

  `_`-prefixed, so import-tree does not pick it up as a module; dashboards
  import it explicitly.

  Datasources are referenced by name rather than by their real uid. Grafana
  resolves a name here through its fallback path — the alert rules under
  ../alerts/ must use the actual uid instead, which is why those two look
  inconsistent. See ../datasources/.
*/
let
  prom = {
    type = "prometheus";
    uid = "Prometheus";
  };
  loki = {
    type = "loki";
    uid = "Loki";
  };

  tsDefaults = {
    color.mode = "palette-classic";
    custom = {
      axisBorderShow = false;
      axisCenteredZero = false;
      axisColorMode = "text";
      axisLabel = "";
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

  greenThresholds = [
    {
      color = "green";
      value = null;
    }
  ];
in
rec {
  inherit
    prom
    loki
    tsDefaults
    tsOptions
    greenThresholds
    ;

  # Red once the value reaches `at` — for panels where any count is a problem.
  redAbove = at: [
    {
      color = "green";
      value = null;
    }
    {
      color = "red";
      value = at;
    }
  ];

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

  # ── stat ──
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
      decimals ? null,
      legendFormat ? "__auto",
      textMode ? "auto",
      colorMode ? "value",
      graphMode ? "none",
      mappings ? [ ],
      description ? "",
      thresholds ? greenThresholds,
    }:
    {
      inherit id title description;
      type = "stat";
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
          inherit mappings unit decimals;
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

  # ── timeseries ──
  ts =
    {
      id,
      title,
      x,
      y,
      w,
      h ? 8,
      unit ? "short",
      targets,
      description ? "",
      drawStyle ? "line",
      fillOpacity ? 10,
      stacking ? "none",
      decimals ? null,
    }:
    {
      inherit
        id
        title
        targets
        description
        ;
      type = "timeseries";
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
          inherit unit decimals;
          custom = tsDefaults.custom // {
            inherit drawStyle fillOpacity;
            stacking = {
              group = "A";
              mode = stacking;
            };
          };
        };
        overrides = [ ];
      };
      options = tsOptions;
    };

  # ── horizontal bar gauge, for "breakdown by label" panels ──
  # Instant query: these read a current level (or an $__range increase), so a
  # time axis would add nothing.
  bar =
    {
      id,
      title,
      x,
      y,
      w,
      h ? 8,
      expr,
      legendFormat,
      unit ? "short",
      description ? "",
      thresholds ? greenThresholds,
    }:
    {
      inherit id title description;
      type = "bargauge";
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
          inherit unit;
          decimals = 0;
          mappings = [ ];
          thresholds = {
            mode = "absolute";
            steps = thresholds;
          };
        };
        overrides = [ ];
      };
      options = {
        displayMode = "gradient";
        orientation = "horizontal";
        showUnfilled = true;
        valueMode = "color";
        reduceOptions = {
          calcs = [ "lastNotNull" ];
          fields = "";
          values = false;
        };
      };
      targets = [ (targetInstant expr legendFormat "A") ];
    };

  # ── logs ──
  # `expr` is expected to select on the service's own systemd unit: the
  # container journal also carries pam, systemd-tmpfiles and Vector's own
  # output, and an unfiltered {host="…"} panel is mostly that noise.
  logs =
    {
      id,
      title,
      x,
      y,
      w ? 24,
      h ? 10,
      expr,
      description ? "",
    }:
    {
      inherit id title description;
      type = "logs";
      gridPos = {
        inherit
          x
          y
          w
          h
          ;
      };
      datasource = loki;
      targets = [
        {
          datasource = loki;
          editorMode = "code";
          inherit expr;
          queryType = "range";
          refId = "A";
        }
      ];
      options = {
        dedupStrategy = "none";
        enableLogDetails = true;
        prettifyLogMessage = false;
        showCommonLabels = false;
        showLabels = false;
        showTime = true;
        sortOrder = "Descending";
        wrapLogMessage = true;
      };
    };

  # ── row separator ──
  row =
    {
      id,
      title,
      y,
    }:
    {
      inherit id title;
      type = "row";
      collapsed = false;
      gridPos = {
        h = 1;
        w = 24;
        x = 0;
        inherit y;
      };
      panels = [ ];
    };
}
