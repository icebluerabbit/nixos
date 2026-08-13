let
  prom = {
    type = "prometheus";
    uid = "Prometheus";
  };
  loki = {
    type = "loki";
    uid = "Loki";
  };
  h = ''host="mousetrap"'';

  # ── panel helpers ─────────────────────────────────────────────────────────
  stat =
    {
      id,
      title,
      expr,
      x,
      w ? 4,
      y ? 0,
      unit ? "short",
      decimals ? 0,
      color ? "value",
      mappings ? [ ],
      legendFormat ? null,
      textMode ? "value",
    }:
    {
      inherit id title;
      type = "stat";
      gridPos = {
        h = 4;
        inherit w x y;
      };
      targets = [
        (
          {
            datasource = prom;
            inherit expr;
            refId = "A";
          }
          // (if legendFormat != null then { inherit legendFormat; } else { })
        )
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = color;
        inherit textMode;
      };
      fieldConfig.defaults = { inherit unit decimals mappings; };
    };

  # horizontal bar-gauge: one bar per series, labelled by legendFormat. Ideal for
  # high-cardinality instant breakdowns (category/filetype/language/reason).
  bar =
    {
      id,
      title,
      expr,
      legendFormat,
      x,
      y,
      w ? 8,
      h ? 8,
      unit ? "short",
      ds ? prom,
    }:
    {
      inherit id title;
      type = "bargauge";
      gridPos = {
        inherit
          h
          w
          x
          y
          ;
      };
      targets = [
        {
          datasource = ds;
          inherit expr legendFormat;
          instant = true;
          refId = "A";
        }
      ];
      options = {
        orientation = "horizontal";
        displayMode = "gradient";
        showUnfilled = true;
        reduceOptions.calcs = [ "lastNotNull" ];
      };
      fieldConfig.defaults = { inherit unit; };
    };

  ts =
    {
      id,
      title,
      targets,
      x,
      y,
      w ? 12,
      h ? 8,
      unit ? "short",
      drawStyle ? "line",
      ds ? prom,
    }:
    {
      inherit id title;
      type = "timeseries";
      gridPos = {
        inherit
          h
          w
          x
          y
          ;
      };
      targets = map (t: t // { datasource = ds; }) targets;
      fieldConfig.defaults = {
        inherit unit;
        custom = {
          inherit drawStyle;
          fillOpacity = if drawStyle == "bars" then 60 else 10;
          lineWidth = 1;
          showPoints = "never";
          stacking.mode = if drawStyle == "bars" then "normal" else "none";
        };
      };
    };

  pie =
    {
      id,
      title,
      expr,
      label,
      x,
      y,
      w ? 8,
      h ? 8,
    }:
    {
      inherit id title;
      type = "piechart";
      gridPos = {
        inherit
          h
          w
          x
          y
          ;
      };
      targets = [
        {
          datasource = prom;
          inherit expr;
          legendFormat = "{{${label}}}";
          instant = true;
          refId = "A";
        }
      ];
      options.legend = {
        displayMode = "table";
        placement = "right";
        values = [ "value" ];
      };
    };

  # instant table of a labelled metric, sorted desc by value. `label` becomes the
  # only visible non-value column; Time and internal fields are hidden.
  table =
    {
      id,
      title,
      expr,
      x,
      y,
      w ? 12,
      h ? 10,
      unit ? "short",
      ds ? prom,
    }:
    {
      inherit id title;
      type = "table";
      gridPos = {
        inherit
          h
          w
          x
          y
          ;
      };
      targets = [
        {
          datasource = ds;
          inherit expr;
          instant = true;
          format = "table";
          refId = "A";
        }
      ];
      transformations = [
        {
          id = "organize";
          options = {
            excludeByName = {
              Time = true;
              job = true;
              instance = true;
              host = true;
            };
            renameByName.Value = "count";
          };
        }
        {
          id = "sortBy";
          options.sort = [
            {
              field = "count";
              desc = true;
            }
          ];
        }
      ];
      fieldConfig.defaults = { inherit unit; };
    };

  # bare logs panel over a LogQL expr.
  logs =
    {
      id,
      title,
      expr,
      x,
      y,
      w ? 24,
      h ? 10,
    }:
    {
      inherit id title;
      type = "logs";
      gridPos = {
        inherit
          h
          w
          x
          y
          ;
      };
      targets = [
        {
          datasource = loki;
          inherit expr;
          refId = "A";
        }
      ];
      options = {
        showTime = true;
        sortOrder = "Descending";
        wrapLogMessage = true;
      };
    };

  # every mousetrap log line is slog JSON on stdout → journald `.message`; parse the
  # wrapper, lift the app line, parse it. Filter on `msg` (slog's message key).
  applog = ''{host="mousetrap"} | json | line_format "{{.message}}" | json'';
in
{
  folder = "Media";
  title = "MyAnonaMouse";
  tags = [
    "media"
    "download"
    "mam"
    "mousetrap"
  ];
  uid = "mousetrap";
  editable = true;
  schemaVersion = 38;
  refresh = "30s";
  time = {
    from = "now-7d";
    to = "now";
  };
  panels = [
    # ══ Account KPIs (y=0) ═══════════════════════════════════════════════════
    (stat {
      id = 1;
      title = "Ratio";
      # clamp +Inf (0 downloaded) so the stat renders; 9999 maps to ∞.
      expr = "clamp_max(mam_ratio{${h}}, 9999)";
      x = 0;
      decimals = 2;
      color = "background";
      mappings = [
        {
          type = "value";
          options."9999".text = "∞";
        }
      ];
    })
    (stat {
      id = 2;
      title = "Buffer";
      expr = "mam_buffer_bytes{${h}}";
      x = 4;
      unit = "bytes";
    })
    (stat {
      id = 3;
      title = "Bonus Points";
      expr = "mam_seedbonus{${h}}";
      x = 8;
    })
    (stat {
      id = 4;
      title = "Class";
      expr = "mam_class_info{${h}}";
      legendFormat = "{{classname}}";
      textMode = "name";
      color = "background";
      x = 12;
    })
    (stat {
      id = 5;
      title = "Wedges";
      expr = "mam_wedges{${h}}";
      x = 16;
      color = "background";
    })
    (stat {
      id = 6;
      title = "Seeding";
      expr = "mam_seeding_count{${h}}";
      x = 20;
    })

    # ══ Account health (y=4) ═════════════════════════════════════════════════
    (stat {
      id = 10;
      title = "Unsatisfied";
      expr = "mam_unsat_count{${h}}";
      x = 0;
      y = 4;
      color = "background";
    })
    (stat {
      id = 11;
      title = "Unsatisfied limit";
      expr = "mam_unsat_limit{${h}}";
      x = 4;
      y = 4;
    })
    (stat {
      id = 12;
      title = "Satisfied";
      expr = "mam_satisfied_count{${h}}";
      x = 8;
      y = 4;
    })
    (stat {
      id = 13;
      title = "Leeching";
      expr = "mam_leeching_count{${h}}";
      x = 12;
      y = 4;
    })
    (stat {
      id = 14;
      title = "Hit-and-Run";
      expr = "mam_hnr_count{${h}}";
      x = 16;
      y = 4;
      color = "background";
    })
    (stat {
      id = 15;
      title = "Refresh age";
      expr = "time() - mam_refresh_timestamp{${h}}";
      x = 20;
      y = 4;
      unit = "s";
      color = "background";
    })

    # ══ Traffic + account detail (y=8) ═══════════════════════════════════════
    (stat {
      id = 20;
      title = "Uploaded";
      expr = "mam_uploaded_bytes{${h}}";
      x = 0;
      y = 8;
      unit = "bytes";
    })
    (stat {
      id = 21;
      title = "Downloaded";
      expr = "mam_downloaded_bytes{${h}}";
      x = 4;
      y = 8;
      unit = "bytes";
    })
    (stat {
      id = 22;
      title = "Seeding unsatisfied";
      expr = "mam_seed_unsat_count{${h}}";
      x = 8;
      y = 8;
    })
    (stat {
      id = 23;
      title = "Seeding HnR";
      expr = "mam_seed_hnr_count{${h}}";
      x = 12;
      y = 8;
    })
    (stat {
      id = 24;
      title = "Inactive HnR";
      expr = "mam_inact_hnr_count{${h}}";
      x = 16;
      y = 8;
    })
    (stat {
      id = 25;
      title = "Bonus ledger rows";
      expr = "mam_bonus_history_count{${h}}";
      x = 20;
      y = 8;
    })

    # ══ In-flight (optimistic) + grab totals (y=12) ══════════════════════════
    (stat {
      id = 30;
      title = "In-flight grabs";
      expr = "mousetrap_inflight_count{${h}}";
      x = 0;
      y = 12;
      color = "background";
    })
    (stat {
      id = 31;
      title = "In-flight ratio-cost bytes";
      expr = "mousetrap_inflight_ratio_bytes{${h}}";
      x = 4;
      y = 12;
      unit = "bytes";
    })
    (stat {
      id = 32;
      title = "In-flight wedges";
      expr = "mousetrap_inflight_wedges{${h}}";
      x = 8;
      y = 12;
    })
    (stat {
      id = 33;
      title = "Grabbed bytes (range)";
      expr = "sum(increase(mousetrap_grab_bytes_total{${h}}[$__range]))";
      x = 12;
      y = 12;
      unit = "bytes";
    })
    (stat {
      id = 34;
      title = "Ratio saved by wedges";
      expr = "sum(increase(mousetrap_wedge_saved_bytes_total{${h}}[$__range]))";
      x = 16;
      y = 12;
      unit = "bytes";
    })
    (stat {
      id = 35;
      title = "Announces (range)";
      expr = "round(sum(increase(mousetrap_announces_total{${h}}[$__range])))";
      x = 20;
      y = 12;
    })

    # ══ ANNOUNCE MIX — the Layer-1 tuning surface (y=16) ═════════════════════
    (table {
      id = 40;
      # Categories and filetypes are multi-valued per release since MAM's 2026-08-11
      # announce change, so these two panels sum ABOVE the announce total — they count
      # (announce, value) pairs, not announces. The by-type panel is one per announce.
      title = "Announces by category (multi-valued)";
      # increase[$__range] reads the whole window from Prometheus' TSDB and stitches
      # across process restarts (in-memory counters reset to 0 on restart); round → ints.
      expr = "round(sum by (category) (increase(mousetrap_announces_by_category_total{${h}}[$__range])))";
      x = 0;
      y = 16;
      w = 12;
      h = 10;
    })
    (bar {
      id = 41;
      title = "By filetype (multi-valued)";
      expr = "sort_desc(round(sum by (filetype) (increase(mousetrap_announces_by_filetype_total{${h}}[$__range]))))";
      legendFormat = "{{filetype}}";
      x = 12;
      y = 16;
      w = 6;
      h = 10;
    })
    (bar {
      id = 42;
      title = "By language";
      expr = "sort_desc(round(sum by (language) (increase(mousetrap_announces_by_language_total{${h}}[$__range]))))";
      legendFormat = "{{language}}";
      x = 18;
      y = 16;
      w = 6;
      h = 10;
    })

    # ══ Type / VIP / freeleech split + bytes (y=26) ══════════════════════════
    (pie {
      id = 50;
      # The `type` label is MAM's media type verbatim ("Ebook", "Audiobook", "Manga",
      # "Periodical Ebook", …) since 2026-08-11 — it was ebook/audiobook/other before,
      # so series break across that date rather than continuing.
      title = "By media type";
      expr = "round(sum by (type) (increase(mousetrap_announces_by_type_total{${h}}[$__range])))";
      label = "type";
      x = 0;
      y = 26;
      w = 6;
      h = 8;
    })
    (pie {
      id = 51;
      title = "By VIP flag";
      expr = "round(sum by (vip) (increase(mousetrap_announces_vip_total{${h}}[$__range])))";
      label = "vip";
      x = 6;
      y = 26;
      w = 6;
      h = 8;
    })
    (pie {
      id = 52;
      title = "By freeleech flag";
      expr = "round(sum by (freeleech) (increase(mousetrap_announces_freeleech_total{${h}}[$__range])))";
      label = "freeleech";
      x = 12;
      y = 26;
      w = 6;
      h = 8;
    })
    (bar {
      id = 53;
      title = "Announced bytes by type";
      expr = "sort_desc(sum by (type) (increase(mousetrap_announce_bytes_total{${h}}[$__range])))";
      legendFormat = "{{type}}";
      x = 18;
      y = 26;
      w = 6;
      h = 8;
      unit = "bytes";
    })

    # ══ Size distribution (y=34) ═════════════════════════════════════════════
    (bar {
      id = 60;
      title = "Avg announce size by type";
      expr = "sort_desc(sum by (type) (increase(mousetrap_announce_size_bytes_sum{${h}}[$__range])) / sum by (type) (increase(mousetrap_announce_size_bytes_count{${h}}[$__range])))";
      legendFormat = "{{type}}";
      x = 0;
      y = 34;
      w = 8;
      h = 8;
      unit = "bytes";
    })
    # cumulative bucket counts as horizontal bars. Read where the mass sits to choose
    # sizeMin/MaxBytes. le labels are byte thresholds.
    (bar {
      id = 61;
      title = "Size distribution";
      expr = "round(sum by (le) (increase(mousetrap_announce_size_bytes_bucket{${h}}[$__range])))";
      legendFormat = "≤ {{le}}";
      x = 8;
      y = 34;
      w = 8;
      h = 8;
    })
    (ts {
      id = 62;
      title = "Announces per hour by type";
      x = 16;
      y = 34;
      w = 8;
      drawStyle = "bars";
      targets = [
        {
          expr = "sum by (type) (increase(mousetrap_announces_by_type_total{${h}}[1h]))";
          legendFormat = "{{type}}";
          refId = "A";
        }
      ];
    })

    # ══ Layer-1 accept decisions (y=42) ══════════════════════════════════════
    (pie {
      id = 70;
      title = "Accept decisions";
      expr = "round(sum by (decision) (increase(mousetrap_would_accept_total{${h}}[$__range])))";
      label = "decision";
      x = 0;
      y = 42;
      w = 8;
      h = 8;
    })
    (table {
      id = 71;
      title = "Reject reasons";
      expr = ''round(sum by (reason) (increase(mousetrap_would_accept_total{${h},decision="reject"}[$__range])))'';
      x = 8;
      y = 42;
      w = 8;
      h = 8;
    })
    (ts {
      id = 72;
      title = "Accept decisions per hour";
      x = 16;
      y = 42;
      w = 8;
      drawStyle = "bars";
      targets = [
        {
          expr = "sum by (decision) (increase(mousetrap_would_accept_total{${h}}[1h]))";
          legendFormat = "{{decision}}";
          refId = "A";
        }
      ];
    })

    # ══ Layer-2 grab gate (y=50) — evaluated on Layer-1 survivors ════════════
    (pie {
      id = 80;
      title = "Grab decisions";
      expr = "round(sum by (decision) (increase(mousetrap_would_grab_total{${h}}[$__range])))";
      label = "decision";
      x = 0;
      y = 50;
      w = 8;
      h = 8;
    })
    (table {
      id = 81;
      title = "Hold reasons";
      expr = ''round(sum by (reason) (increase(mousetrap_would_grab_total{${h},decision="hold"}[$__range])))'';
      x = 8;
      y = 50;
      w = 8;
      h = 8;
    })
    # grab ACTION outcome (dryrun until armed; then added/added_wedge/fetch_failed/…).
    (table {
      id = 82;
      title = "Grab results";
      expr = "round(sum by (result) (increase(mousetrap_grab_action_total{${h}}[$__range])))";
      x = 16;
      y = 50;
      w = 8;
      h = 8;
    })

    # ══ Grabs — source, cadence, latency (y=58) ══════════════════════════════
    (pie {
      id = 90;
      title = "Grabs by source";
      expr = "round(sum by (source) (increase(mousetrap_grabs_by_source_total{${h}}[$__range])))";
      label = "source";
      x = 0;
      y = 58;
      w = 8;
      h = 8;
    })
    (ts {
      id = 91;
      title = "Grabs per hour by result";
      x = 8;
      y = 58;
      w = 8;
      drawStyle = "bars";
      targets = [
        {
          expr = "sum by (result) (increase(mousetrap_grab_action_total{${h}}[1h]))";
          legendFormat = "{{result}}";
          refId = "A";
        }
      ];
    })
    (ts {
      id = 92;
      title = "Grab duration";
      x = 16;
      y = 58;
      w = 8;
      unit = "s";
      targets = [
        {
          expr = "histogram_quantile(0.5, sum by (le) (rate(mousetrap_grab_duration_seconds_bucket{${h}}[$__rate_interval])))";
          legendFormat = "p50";
          refId = "A";
        }
        {
          expr = "histogram_quantile(0.95, sum by (le) (rate(mousetrap_grab_duration_seconds_bucket{${h}}[$__rate_interval])))";
          legendFormat = "p95";
          refId = "B";
        }
      ];
    })

    # ══ Spends + bonus ledger (y=66) ═════════════════════════════════════════
    (table {
      id = 100;
      title = "Spend events by reason";
      expr = "round(sum by (reason) (increase(mousetrap_spends_total{${h}}[$__range])))";
      x = 0;
      y = 66;
      w = 8;
      h = 8;
    })
    (pie {
      id = 101;
      title = "Spend events by type";
      expr = "round(sum by (type) (increase(mousetrap_spends_by_type_total{${h}}[$__range])))";
      label = "type";
      x = 8;
      y = 66;
      w = 8;
      h = 8;
    })
    (ts {
      id = 102;
      title = "Bonus events per hour by type";
      x = 16;
      y = 66;
      w = 8;
      h = 8;
      drawStyle = "bars";
      targets = [
        {
          expr = "sum by (type) (increase(mousetrap_bonus_received_total{${h}}[1h]))";
          legendFormat = "{{type}}";
          refId = "A";
        }
      ];
    })

    # ══ Spend/bonus amounts + errors (y=74) ══════════════════════════════════
    (bar {
      id = 110;
      title = "Spent amount by type";
      expr = "round(sum by (type) (increase(mousetrap_spend_amount_total{${h}}[$__range])))";
      legendFormat = "{{type}}";
      x = 0;
      y = 74;
      w = 8;
      h = 8;
    })
    (bar {
      id = 111;
      title = "Bonus amount by type";
      expr = "round(sum by (type) (increase(mousetrap_bonus_amount_total{${h}}[$__range])))";
      legendFormat = "{{type}}";
      x = 8;
      y = 74;
      w = 8;
      h = 8;
    })
    (table {
      id = 112;
      title = "MAM API errors by op";
      expr = "round(sum by (op) (increase(mousetrap_mam_errors_total{${h}}[$__range])))";
      x = 16;
      y = 74;
      w = 8;
      h = 8;
    })

    # ══ Logs (y=82) — aligned to the Go slog fields (msg=, flat keys) ════════
    (logs {
      id = 120;
      title = "Recent announces";
      # slog "announce" line: decision/reason + grab_decision/grab_reason/grab_result,
      # plus media type + categories + name. mam_id never appears in these attrs.
      # `categories` is logged pre-joined ("Crime, Paranormal") because Loki's json
      # parser flattens an array into categories_0/categories_1, unusable here.
      # derive a `level` label so Grafana colors the gutter: grabbed=green, held=yellow,
      # fetch/add failure=red, plain layer-1 reject=grey.
      expr = ''${applog} | msg="announce" | label_format level=`{{if or (eq .grab_result "fetch_failed") (eq .grab_result "add_failed") (eq .grab_result "add_rejected")}}error{{else if or (eq .grab_result "added") (eq .grab_result "added_wedge")}}info{{else if eq .grab_decision "hold"}}warn{{else}}debug{{end}}` | line_format "{{.decision}}/{{.reason}}  {{.grab_decision}}/{{.grab_reason}}  {{.grab_result}}  [{{.media_type}}/{{.fiction}}: {{.categories}}]  {{.name}}"'';
      x = 0;
      y = 82;
      w = 24;
      h = 10;
    })
    (logs {
      id = 121;
      title = "Recent errors (grab / refresh / MAM)";
      expr = ''${applog} | level="ERROR" | line_format "{{.msg}}  tid={{.tid}}  {{.endpoint}}{{.stage}}  code={{.code}}  {{.err}}"'';
      x = 0;
      y = 92;
      w = 24;
      h = 8;
    })
  ];
}
