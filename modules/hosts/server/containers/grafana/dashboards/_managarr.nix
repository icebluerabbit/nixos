# managarr — the whole stack as one picture.
#
# Every other dashboard here reports one service's own opinion of itself.
# managarr is the only source that has joined them, so this is where questions
# that span services get answered: what was announced but never grabbed, which
# torrents are below a tracker's ratio rule, whether a tagged music file can be
# traced back to the search that found it.
#
# Two metric families feed it and they answer different questions:
#
#   managarr_*            the STACK — counts of rows, refreshed on a one-minute
#                         ticker against Postgres
#   managarr_reconcile_*  the PASSES — whether the recorder itself is working
#   sidecar_*             the sidecars, scraped inside each service's container
#
# Gauges throughout for the first family, because each is a snapshot of a table
# rather than an accumulating count. `delta()` over a range is what turns one
# into a rate; `deriv()` is wrong here and would report fractional rows.
let
  prom = {
    type = "prometheus";
    uid = "Prometheus";
  };

  # Panel builders. The dashboards in this folder are hand-written attrsets and
  # stay that way, but a seven-row dashboard is ~40 panels and repeating the
  # same eight keys forty times is how a gridPos ends up overlapping.
  stat =
    {
      id,
      title,
      expr,
      x,
      y,
      w ? 4,
      h ? 4,
      unit ? "short",
      decimals ? 0,
      legend ? "",
    }:
    {
      inherit title id;
      type = "stat";
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
          refId = "A";
          legendFormat = legend;
        }
      ];
      options = {
        reduceOptions.calcs = [ "lastNotNull" ];
        colorMode = "background";
        graphMode = "area";
      };
      fieldConfig.defaults = { inherit unit decimals; };
    };

  ts =
    {
      id,
      title,
      expr,
      x,
      y,
      w ? 12,
      h ? 8,
      unit ? "short",
      legend ? "{{__name__}}",
      stack ? false,
      desc ? "",
    }:
    {
      inherit title id;
      type = "timeseries";
      description = desc;
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
          refId = "A";
          legendFormat = legend;
        }
      ];
      fieldConfig.defaults = {
        inherit unit;
        custom = {
          fillOpacity = if stack then 60 else 10;
          showPoints = "never";
          lineWidth = 1;
          stacking.mode = if stack then "normal" else "none";
        };
      };
      options.legend = {
        displayMode = "table";
        placement = "right";
        calcs = [
          "lastNotNull"
          "max"
        ];
      };
    };

  bar =
    {
      id,
      title,
      expr,
      x,
      y,
      w ? 12,
      h ? 8,
      legend ? "{{__name__}}",
      desc ? "",
    }:
    {
      inherit title id;
      type = "barchart";
      description = desc;
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
          refId = "A";
          legendFormat = legend;
          instant = true;
        }
      ];
      options.legend.displayMode = "list";
      fieldConfig.defaults.custom.fillOpacity = 80;
    };

  table =
    {
      id,
      title,
      expr,
      x,
      y,
      w ? 12,
      h ? 8,
      desc ? "",
    }:
    {
      inherit title id;
      type = "table";
      description = desc;
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
          refId = "A";
          instant = true;
          format = "table";
        }
      ];
      options.sortBy = [
        {
          displayName = "Value";
          desc = true;
        }
      ];
    };

  row =
    {
      id,
      title,
      y,
    }:
    {
      inherit title id;
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
in
{
  folder = "Media";
  title = "managarr";
  tags = [
    "media"
    "managarr"
    "provenance"
  ];
  uid = "managarr";
  editable = true;
  schemaVersion = 38;
  refresh = "1m";
  time = {
    from = "now-7d";
    to = "now";
  };

  panels = [
    # ══ overview ═══════════════════════════════════════════════════════════
    (row {
      id = 100;
      title = "Overview";
      y = 0;
    })
    (stat {
      id = 1;
      title = "Library files";
      expr = "sum(managarr_library_files)";
      x = 0;
      y = 1;
    })
    (stat {
      id = 2;
      title = "Catalog entries";
      expr = "sum(managarr_contents)";
      x = 4;
      y = 1;
    })
    (stat {
      id = 3;
      title = "Torrents held";
      expr = "sum(managarr_client_torrents)";
      x = 8;
      y = 1;
    })
    (stat {
      id = 4;
      title = "Below ratio 1.0";
      expr = ''sum(managarr_client_torrents_by_ratio{band=~"under_0.5|under_1"})'';
      x = 12;
      y = 1;
    })
    (stat {
      id = 5;
      title = "Uploaded";
      expr = "sum(managarr_client_uploaded_bytes)";
      x = 16;
      y = 1;
      unit = "bytes";
    })
    (stat {
      id = 6;
      title = "Share ratio";
      expr = "sum(managarr_client_uploaded_bytes) / sum(managarr_client_downloaded_bytes)";
      x = 20;
      y = 1;
      decimals = 2;
    })

    # ══ acquisition funnel ═════════════════════════════════════════════════
    (row {
      id = 101;
      title = "Acquisition funnel";
      y = 5;
    })
    (ts {
      id = 10;
      title = "Decisions by outcome";
      expr = "sum by (outcome) (managarr_decisions)";
      legend = "{{outcome}}";
      x = 0;
      y = 6;
      stack = true;
      desc = ''
        Every verdict anything in the stack reached. Rejections are the
        majority and exist nowhere else — a release a filter declined never
        reaches a *arr, a client or a tracker's history.
      '';
    })
    (ts {
      id = 11;
      title = "Decisions by decider";
      expr = "sum by (decider) (managarr_decisions)";
      legend = "{{decider}}";
      x = 12;
      y = 6;
      desc = "Who is doing the deciding: autobrr's filters, a *arr's quality profile, cross-seed's matcher.";
    })
    (bar {
      id = 12;
      title = "Rejections by decider";
      expr = ''sum by (decider) (managarr_decisions{outcome="rejected"})'';
      legend = "{{decider}}";
      x = 0;
      y = 14;
      desc = "Where the funnel leaks. A decider rejecting far more than it accepts is either working correctly or misconfigured, and this is the only place the two look different.";
    })
    (bar {
      id = 13;
      title = "Offers by indexer";
      expr = "sum by (indexer) (managarr_offers)";
      legend = "{{indexer}}";
      x = 12;
      y = 14;
    })
    (ts {
      id = 14;
      title = "Grabs";
      expr = "sum by (service) (managarr_grabs)";
      legend = "{{service}}";
      x = 0;
      y = 22;
    })
    (table {
      id = 15;
      title = "Import failures";
      expr = "sum by (service, kind) (managarr_import_failures)";
      x = 12;
      y = 22;
      desc = "A grab that never became a file. The kind says why, and that reason expires from the *arr's own queue within minutes.";
    })

    # ══ seeding ════════════════════════════════════════════════════════════
    (row {
      id = 102;
      title = "Seeding health";
      y = 30;
    })
    (ts {
      id = 20;
      title = "Torrents by state";
      expr = "sum by (state) (managarr_client_torrents)";
      legend = "{{state}}";
      x = 0;
      y = 31;
      stack = true;
    })
    (ts {
      id = 21;
      title = "Ratio distribution";
      expr = "sum by (band) (managarr_client_torrents_by_ratio)";
      legend = "{{band}}";
      x = 12;
      y = 31;
      stack = true;
      desc = ''
        The only part of this model with money attached: a tracker enforces its
        hit-and-run rule on ratio and seeding time, and nothing else reports
        either across every client at once.
      '';
    })
    (ts {
      id = 22;
      title = "Transfer totals";
      expr = "sum(managarr_client_uploaded_bytes)";
      legend = "uploaded";
      x = 0;
      y = 39;
      unit = "bytes";
    })
    (stat {
      id = 23;
      title = "Seeding time";
      expr = "sum(managarr_client_seeding_seconds)";
      x = 12;
      y = 39;
      w = 6;
      unit = "s";
    })
    (stat {
      id = 24;
      title = "Downloaded";
      expr = "sum(managarr_client_downloaded_bytes)";
      x = 18;
      y = 39;
      w = 6;
      unit = "bytes";
    })

    # ══ library ════════════════════════════════════════════════════════════
    (row {
      id = 103;
      title = "Library";
      y = 47;
    })
    (bar {
      id = 30;
      title = "Catalog by kind";
      expr = "sum by (kind) (managarr_contents)";
      legend = "{{kind}}";
      x = 0;
      y = 48;
    })
    (bar {
      id = 31;
      title = "Files by resolution";
      expr = "sum by (resolution) (managarr_library_files_by_resolution)";
      legend = "{{resolution}}";
      x = 12;
      y = 48;
    })
    (ts {
      id = 32;
      title = "Subtitles present";
      expr = "sum by (language) (managarr_subtitle_files)";
      legend = "{{language}}";
      x = 0;
      y = 56;
    })
    (ts {
      id = 33;
      title = "Subtitles wanted";
      expr = "sum by (language) (managarr_subtitle_wanted)";
      legend = "{{language}}";
      x = 12;
      y = 56;
      desc = ''
        The only absence in this model that can be counted. A subtitle that was
        never downloaded leaves nothing on disk, so Bazarr is the sole witness
        — and this number going UP is the library growing, not a regression.
      '';
    })

    # ══ music ══════════════════════════════════════════════════════════════
    (row {
      id = 104;
      title = "Music";
      y = 64;
    })
    (ts {
      id = 40;
      title = "Soulseek downloads";
      expr = "sum by (state) (managarr_soulseek_downloads)";
      legend = "{{state}}";
      x = 0;
      y = 65;
      stack = true;
    })
    (ts {
      id = 41;
      title = "Soulseek uploads";
      expr = "sum by (state) (managarr_soulseek_uploads)";
      legend = "{{state}}";
      x = 12;
      y = 65;
      stack = true;
      desc = "What this homelab has shared. Lives in slskd's event log and nowhere else — its transfers API expires a completed entry.";
    })
    (bar {
      id = 42;
      title = "Tagged files resolved to the catalog";
      expr = "sum by (matched) (managarr_beets_items)";
      legend = "matched={{matched}}";
      x = 0;
      y = 73;
      desc = ''
        The music loop closing. beets' mb_releasegroupid is what Lidarr calls
        foreignAlbumId, so a matched item is a file whose provenance runs all
        the way back to the search that found it. An unmatched one is a file
        the library cannot explain.
      '';
    })

    # ══ indexers ═══════════════════════════════════════════════════════════
    (row {
      id = 105;
      title = "Indexers";
      y = 81;
    })
    (bar {
      id = 50;
      title = "Searches by indexer";
      expr = "sum by (indexer) (managarr_searches)";
      legend = "{{indexer}}";
      x = 0;
      y = 82;
      desc = "Prowlarr proxies every query in this stack, which makes it the only record of what was looked FOR rather than what was taken.";
    })
    (bar {
      id = 51;
      title = "Mean search latency";
      expr = "managarr_search_elapsed_ms_avg";
      legend = "{{indexer}}";
      x = 12;
      y = 82;
    })
    (bar {
      id = 52;
      title = "Grabs by indexer";
      expr = "sum by (indexer) (managarr_indexer_grabs)";
      legend = "{{indexer}}";
      x = 0;
      y = 90;
    })

    # ══ operational ════════════════════════════════════════════════════════
    (row {
      id = 106;
      title = "Recorder health";
      y = 98;
    })
    (ts {
      id = 60;
      title = "Reconcile duration";
      expr = "histogram_quantile(0.9, sum by (le, pass) (rate(managarr_reconcile_duration_seconds_bucket[$__rate_interval])))";
      legend = "{{pass}}";
      x = 0;
      y = 99;
      unit = "s";
    })
    (table {
      id = 61;
      title = "Time since last successful pass";
      expr = "time() - managarr_reconcile_last_success_timestamp_seconds";
      x = 12;
      y = 99;
      desc = ''
        Zero means a pass has never succeeded since this process started — the
        series is pre-registered at startup precisely so this panel is not
        empty after a deploy.
      '';
    })
    (ts {
      id = 62;
      title = "Pass failures";
      expr = "sum by (pass) (increase(managarr_reconcile_failures_total[$__rate_interval]))";
      legend = "{{pass}}";
      x = 0;
      y = 107;
    })
    (ts {
      id = 63;
      title = "Sidecar cursor lag";
      expr = "managarr_sidecar_cursor_lag_seconds";
      legend = "{{source}}";
      x = 12;
      y = 107;
      unit = "s";
      desc = ''
        Age of the newest event each pass has read. Flat and low is healthy.
        Growing without bound means a pass has stopped consuming its sidecar —
        which nothing else reports, because the pass still succeeds.
      '';
    })
    (ts {
      id = 64;
      title = "Events read through cursors";
      expr = "managarr_sidecar_cursor_events_read";
      legend = "{{source}}";
      x = 0;
      y = 115;
      desc = ''
        Cumulative. A step equal to the whole corpus every hour means a cursor
        is resetting and the incremental read is not working — which otherwise
        looks identical to a healthy pass, just slower.
      '';
    })
    (ts {
      id = 65;
      title = "Sidecar events relayed";
      expr = "sum by (source) (rate(sidecar_events_relayed_total[$__rate_interval]))";
      legend = "{{source}}";
      x = 12;
      y = 115;
      desc = "The push half. qBittorrent is the one that matters: it keeps no history, so an event missed here is gone rather than late.";
    })
    (ts {
      id = 66;
      title = "Sidecar events failed";
      expr = "sum by (source, stage) (rate(sidecar_events_failed_total[$__rate_interval]))";
      legend = "{{source}} {{stage}}";
      x = 0;
      y = 123;
      desc = "stage says which leg broke: decode, enrich (the upstream would not answer) or relay (managarr would not take it).";
    })
    (stat {
      id = 67;
      title = "Stats last refreshed";
      expr = "time() - managarr_stats_refreshed_timestamp_seconds";
      x = 12;
      y = 123;
      w = 6;
      unit = "s";
      decimals = 0;
    })
    (stat {
      id = 68;
      title = "Stats query failures";
      expr = "sum(increase(managarr_stats_failures_total[$__range]))";
      x = 18;
      y = 123;
      w = 6;
    })
  ];
}
