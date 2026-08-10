/*
  Cleanuparr.

  Every series here is scraped from Cleanuparr's own `/api/v2/stats` by the
  Vector sidecar (../../cleanuparr/_exporter.nix) — no log parsing is involved.

  READ THIS BEFORE TRUSTING A NUMBER: the API aggregates over a rolling window,
  reported as `timeframeHours` (168h / 7d). Every count below is therefore
  "how many in the last week", a level that rises and falls, not a cumulative
  counter. `rate()` and `increase()` over them are meaningless. That is also why
  the panels use `now-7d` and why the window is shown as its own stat — if that
  tile ever reads something other than 168, every count on this dashboard has
  silently changed meaning.

  Cleanuparr is in phase 0 (see ../../cleanuparr/cleanuparr.nix): tagging runs,
  nothing strikes, nothing is deleted. So `Removals` and `Cleaned` are expected
  to be flat, and the interesting panel is the event breakdown, where
  CategoryChanged carries the tagging traffic. When phase 1 flips the strike
  thresholds, the strike panels are what should start moving.
*/
let
  p = import ./_panels.nix;
  inherit (p)
    stat
    ts
    bar
    logs
    row
    target
    redAbove
    ;
in
{
  folder = "Media";
  title = "Cleanuparr";
  uid = "cleanuparr";
  description = "Cleanuparr: queue/download cleanup activity, strikes and their outcomes, per-job schedule freshness, and the health of every *arr and download client it talks to. Scraped from /api/v2/stats — all counts are over a rolling 7-day window.";
  editable = true;
  graphTooltip = 1;
  schemaVersion = 40;
  refresh = "1m";
  timezone = "browser";
  tags = [
    "media"
    "download"
    "cleanuparr"
  ];

  time = {
    from = "now-7d";
    to = "now";
  };

  templating.list = [
    {
      name = "level";
      label = "Log level";
      type = "custom";
      query = "ERR,WRN,INF,DBG";
      multi = true;
      includeAll = true;
      allValue = ".*";
      current = {
        selected = true;
        text = [ "All" ];
        value = [ "$__all" ];
      };
      options = [ ];
    }
  ];

  panels = [
    (row {
      id = 100;
      title = "Activity — rolling window";
      y = 0;
    })
    (stat {
      id = 1;
      title = "Events";
      x = 0;
      y = 1;
      w = 3;
      expr = "cleanuparr_events";
      decimals = 0;
      graphMode = "area";
    })
    (stat {
      id = 2;
      title = "Strikes";
      x = 3;
      y = 1;
      w = 3;
      expr = "cleanuparr_strikes";
      decimals = 0;
      graphMode = "area";
    })
    (stat {
      id = 3;
      title = "Strikes recovered";
      x = 6;
      y = 1;
      w = 3;
      expr = "cleanuparr_strikes_recovered";
      decimals = 0;
      description = "Downloads that were struck and then made progress again, clearing the strike. A high share means the thresholds are impatient rather than the downloads bad.";
    })
    (stat {
      id = 4;
      title = "Removals";
      x = 9;
      y = 1;
      w = 3;
      expr = "cleanuparr_removals";
      decimals = 0;
      description = "Items pulled from an *arr queue and blocklisted. Expected to be flat in phase 0.";
    })
    (stat {
      id = 5;
      title = "Cleaned";
      x = 12;
      y = 1;
      w = 3;
      expr = "cleanuparr_cleaned";
      decimals = 0;
      description = "Downloads removed by the seeding/orphan rules. Zero until phase 2 — this is the only thing here that deletes data.";
    })
    (stat {
      id = 6;
      title = "Searches grabbed";
      x = 15;
      y = 1;
      w = 3;
      expr = "cleanuparr_searches_grabbed";
      decimals = 0;
    })
    (stat {
      id = 7;
      title = "Searches failed";
      x = 18;
      y = 1;
      w = 3;
      expr = "cleanuparr_searches_failed";
      decimals = 0;
      colorMode = "background";
      thresholds = redAbove 1;
    })
    (stat {
      id = 8;
      title = "Stats window";
      x = 21;
      y = 1;
      w = 3;
      expr = "cleanuparr_stats_window_hours";
      unit = "h";
      decimals = 0;
      description = "The API's timeframeHours. Every count on this dashboard is scoped to it. If this stops reading 168, the numbers here changed meaning.";
    })

    (row {
      id = 101;
      title = "What it is doing";
      y = 5;
    })
    (bar {
      id = 10;
      title = "Events by type";
      x = 0;
      y = 6;
      w = 12;
      expr = "sort_desc(cleanuparr_events_by_type)";
      legendFormat = "{{type}}";
      description = "In phase 0 this should be dominated by CategoryChanged — that is the unlinked/dead tagging. Strike event types appearing here means something is striking that should not be.";
    })
    (ts {
      id = 11;
      title = "Event level over time";
      x = 12;
      y = 6;
      w = 12;
      targets = [
        (target "cleanuparr_events_by_type" "{{type}}" "A")
      ];
      decimals = 0;
      description = "The same rolling-window levels plotted over time. A rising line means events of that type are arriving faster than they age out of the window.";
    })
    (bar {
      id = 12;
      title = "Strikes by type";
      x = 0;
      y = 14;
      w = 8;
      expr = "sort_desc(cleanuparr_strikes_by_type)";
      legendFormat = "{{type}}";
    })
    (bar {
      id = 13;
      title = "Removals by reason";
      x = 8;
      y = 14;
      w = 8;
      expr = "sort_desc(cleanuparr_removals_by_reason)";
      legendFormat = "{{reason}}";
    })
    (bar {
      id = 14;
      title = "Events by severity";
      x = 16;
      y = 14;
      w = 8;
      expr = "sort_desc(cleanuparr_events_by_severity)";
      legendFormat = "{{severity}}";
    })

    (row {
      id = 102;
      title = "Jobs";
      y = 22;
    })
    (stat {
      id = 20;
      title = "Since last run";
      x = 0;
      y = 23;
      w = 12;
      h = 5;
      expr = ''time() - cleanuparr_job_run_timestamp_seconds{which="last"}'';
      legendFormat = "{{job}}";
      unit = "s";
      textMode = "value_and_name";
      colorMode = "value";
      description = "QueueCleaner runs every 5m, DownloadCleaner every 6h. A job climbing past its interval has stopped running. Jobs that have never run publish no series and are absent here.";
    })
    (stat {
      id = 21;
      title = "Next run in";
      x = 12;
      y = 23;
      w = 12;
      h = 5;
      expr = ''cleanuparr_job_run_timestamp_seconds{which="next"} - time()'';
      legendFormat = "{{job}}";
      unit = "s";
      textMode = "value_and_name";
      colorMode = "value";
      description = "Jobs with no scheduled next occurrence (Seeker, with search disabled) are absent rather than shown as zero.";
    })
    (ts {
      id = 22;
      title = "Job runs in window";
      x = 0;
      y = 28;
      w = 12;
      targets = [
        (target "cleanuparr_job_runs" "{{job}}" "A")
      ];
      decimals = 0;
    })
    (ts {
      id = 23;
      title = "Failed job runs";
      x = 12;
      y = 28;
      w = 12;
      targets = [
        (target "cleanuparr_job_runs_failed" "{{job}}" "A")
      ];
      decimals = 0;
      description = "Non-zero here means a job is erroring out, which silently stops the cleanup it was meant to do.";
    })

    (row {
      id = 103;
      title = "Connected services";
      y = 36;
    })
    (stat {
      id = 30;
      title = "Service health";
      x = 0;
      y = 37;
      w = 12;
      h = 5;
      expr = "cleanuparr_service_healthy";
      legendFormat = "{{service}}";
      textMode = "value_and_name";
      colorMode = "background";
      mappings = [
        {
          type = "value";
          options = {
            "0" = {
              color = "red";
              index = 0;
              text = "Down";
            };
            "1" = {
              color = "green";
              index = 1;
              text = "Up";
            };
          };
        }
      ];
      description = "Both the download clients and the *arr instances Cleanuparr is configured against. An unreachable *arr means its queue simply stops being cleaned, with no other symptom.";
    })
    (ts {
      id = 31;
      title = "Download client response time";
      x = 12;
      y = 37;
      w = 12;
      h = 5;
      targets = [
        (target "cleanuparr_client_response_seconds" "{{client}}" "A")
      ];
      unit = "s";
      description = "qBittorrent's answer latency as Cleanuparr measures it. This sits in the tens of seconds with a large torrent list, and a climb here is the first sign the client is struggling before anything else notices.";
    })

    (row {
      id = 104;
      title = "Logs";
      y = 42;
    })
    (logs {
      id = 40;
      title = "Cleanuparr log";
      x = 0;
      y = 43;
      expr = ''{host="cleanuparr"} | json | _SYSTEMD_UNIT = "cleanuparr.service" | line_format "{{.message}}" |~ "\\[(?i)$level\\]"'';
      description = "Filtered to the cleanuparr unit. Serilog writes the level as [INF]/[WRN]/[ERR], which the $level variable matches on.";
    })
  ];
}
