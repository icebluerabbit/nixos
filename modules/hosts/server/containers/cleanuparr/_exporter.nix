/*
  Vector metrics for cleanuparr, scraped from its own REST API.

  Cleanuparr serves a /metrics path, but it is the Angular SPA's index.html —
  there is no Prometheus endpoint. What it does have is `/api/v2/stats`, which
  already aggregates everything a dashboard wants, so nothing here parses a log
  line:

    events   .total .byType{CategoryChanged,FailedImportStrike,StalledStrike,…}
                    .bySeverity{Important,Information}
    strikes  .total .recovered .byType{Stalled,FailedImport,DeadTorrent}
    removals .total .byReason{FailedImport,Stalled}
    cleaned  .total .byReason
    searches .total .completed .failed .grabbed .byReason
    jobs     .byType{QueueCleaner,DownloadCleaner,Seeker,…}
             → .total .completed .failed .lastRunAt .nextRunAt
    health   .downloadClients[] .arrInstances[]
             → .name .type .isHealthy .lastChecked [.responseTimeMs]

  IMPORTANT: everything the response counts is scoped to a rolling window —
  `timeframeHours` comes back as 168, i.e. the last 7 days, not since startup.
  These are therefore gauges of "how many in the last week" and are named
  without the `_total` suffix, which in Prometheus means a cumulative counter.
  `increase()`/`rate()` over them is meaningless; read them as levels.

  `/api/health` covers the same ground for download clients only, and reports
  its response time as a .NET TimeSpan string rather than a number, so the
  health block above is used in preference and that endpoint is left alone.

  No key is needed: `general.auth.disable_auth_for_local_addresses` is on (see
  cleanuparr.nix) and the sidecar scrapes over loopback.

  Prometheus wants one series per key of those byType/byReason maps, and the
  way to fan one Vector event out into many is a remap whose `.` ends up an
  array — there is no unnest transform. Each *_parse below therefore emits one
  event per {key,value} pair. Walking the map rather than naming its keys here
  means a new upstream event, strike or job type appears on its own instead of
  needing this file edited.
*/
{ port }:
let
  base = "http://127.0.0.1:${toString port}";

  # One (parse → gauge) chain per {name: count} map in /api/v2/stats. `path` is
  # the VRL path to the map, `label` the Prometheus label its keys become.
  # Re-parsing the response per map costs nothing at a 60s scrape and keeps
  # each chain readable in isolation.
  mapMetric =
    {
      id,
      path,
      name,
      label,
    }:
    {
      "cleanuparr_${id}_parse" = {
        type = "remap";
        inputs = [ "cleanuparr_stats" ];
        drop_on_abort = true;
        source = ''
          s = parse_json!(string!(.message))
          acc = []
          for_each(object(${path}) ?? {}) -> |k, v| {
            acc = push(acc, { "key": k, "value": to_float(v) ?? 0.0 })
          }
          . = acc
        '';
      };
      "cleanuparr_${id}_metric" = {
        type = "log_to_metric";
        inputs = [ "cleanuparr_${id}_parse" ];
        metrics = [
          {
            type = "gauge";
            field = "value";
            inherit name;
            tags.${label} = "{{key}}";
          }
        ];
      };
    };

  maps = [
    {
      id = "eventtypes";
      path = "s.events.byType";
      name = "cleanuparr_events_by_type";
      label = "type";
    }
    {
      id = "eventseverity";
      path = "s.events.bySeverity";
      name = "cleanuparr_events_by_severity";
      label = "severity";
    }
    {
      id = "striketypes";
      path = "s.strikes.byType";
      name = "cleanuparr_strikes_by_type";
      label = "type";
    }
    {
      id = "removalreasons";
      path = "s.removals.byReason";
      name = "cleanuparr_removals_by_reason";
      label = "reason";
    }
    {
      id = "cleanedreasons";
      path = "s.cleaned.byReason";
      name = "cleanuparr_cleaned_by_reason";
      label = "reason";
    }
    {
      id = "searchreasons";
      path = "s.searches.byReason";
      name = "cleanuparr_searches_by_reason";
      label = "reason";
    }
  ];
in
{
  prometheusInputs = [
    "cleanuparr_totals_metric"
    "cleanuparr_jobs_metric"
    "cleanuparr_jobschedule_metric"
    "cleanuparr_health_metric"
    "cleanuparr_latency_metric"
  ]
  ++ map (m: "cleanuparr_${m.id}_metric") maps;

  extraSettings = {
    sources.cleanuparr_stats = {
      type = "http_client";
      endpoint = "${base}/api/v2/stats";
      scrape_interval_secs = 60;
      scrape_timeout_secs = 30;
    };

    transforms = {
      # ── Window-wide scalars ─────────────────────────────────────────────
      # Kept alongside the _by_ breakdowns rather than left to sum(): an empty
      # map (cleaned.byReason is {} until phase 2) publishes no series at all,
      # and a KPI tile reading "No data" when the real answer is 0 is worse
      # than a redundant series.
      cleanuparr_totals_parse = {
        type = "remap";
        inputs = [ "cleanuparr_stats" ];
        drop_on_abort = true;
        source = ''
          s = parse_json!(string!(.message))
          . = {
            "events": to_float(s.events.total) ?? 0.0,
            "strikes": to_float(s.strikes.total) ?? 0.0,
            "strikes_recovered": to_float(s.strikes.recovered) ?? 0.0,
            "removals": to_float(s.removals.total) ?? 0.0,
            "cleaned": to_float(s.cleaned.total) ?? 0.0,
            "searches": to_float(s.searches.total) ?? 0.0,
            "searches_completed": to_float(s.searches.completed) ?? 0.0,
            "searches_failed": to_float(s.searches.failed) ?? 0.0,
            "searches_grabbed": to_float(s.searches.grabbed) ?? 0.0,
            "window_hours": to_float(s.timeframeHours) ?? 0.0,
          }
        '';
      };
      cleanuparr_totals_metric = {
        type = "log_to_metric";
        inputs = [ "cleanuparr_totals_parse" ];
        metrics = [
          {
            type = "gauge";
            field = "events";
            name = "cleanuparr_events";
          }
          {
            type = "gauge";
            field = "strikes";
            name = "cleanuparr_strikes";
          }
          {
            type = "gauge";
            field = "strikes_recovered";
            name = "cleanuparr_strikes_recovered";
          }
          {
            type = "gauge";
            field = "removals";
            name = "cleanuparr_removals";
          }
          {
            type = "gauge";
            field = "cleaned";
            name = "cleanuparr_cleaned";
          }
          {
            type = "gauge";
            field = "searches";
            name = "cleanuparr_searches";
          }
          {
            type = "gauge";
            field = "searches_completed";
            name = "cleanuparr_searches_completed";
          }
          {
            type = "gauge";
            field = "searches_failed";
            name = "cleanuparr_searches_failed";
          }
          {
            type = "gauge";
            field = "searches_grabbed";
            name = "cleanuparr_searches_grabbed";
          }
          {
            type = "gauge";
            field = "window_hours";
            name = "cleanuparr_stats_window_hours";
          }
        ];
      };

      # ── Per-job run counts ──────────────────────────────────────────────
      cleanuparr_jobs_parse = {
        type = "remap";
        inputs = [ "cleanuparr_stats" ];
        drop_on_abort = true;
        source = ''
          s = parse_json!(string!(.message))
          acc = []
          for_each(object(s.jobs.byType) ?? {}) -> |name, j| {
            acc = push(acc, {
              "key": name,
              "total": to_float(j.total) ?? 0.0,
              "completed": to_float(j.completed) ?? 0.0,
              "failed": to_float(j.failed) ?? 0.0,
            })
          }
          . = acc
        '';
      };
      cleanuparr_jobs_metric = {
        type = "log_to_metric";
        inputs = [ "cleanuparr_jobs_parse" ];
        metrics = [
          {
            type = "gauge";
            field = "total";
            name = "cleanuparr_job_runs";
            tags.job = "{{key}}";
          }
          {
            type = "gauge";
            field = "completed";
            name = "cleanuparr_job_runs_completed";
            tags.job = "{{key}}";
          }
          {
            type = "gauge";
            field = "failed";
            name = "cleanuparr_job_runs_failed";
            tags.job = "{{key}}";
          }
        ];
      };

      # ── Job schedule freshness ──────────────────────────────────────────
      # Absolute epochs, so a panel shows `time() - metric` for "how long since"
      # and `metric - time()` for "due in". nextRunAt is null for a job with no
      # next occurrence (Seeker, with search disabled) and lastRunAt is null
      # before a job's first run; those pairs are skipped, because an absent
      # series reads better on a panel than a sentinel every query must filter.
      cleanuparr_jobschedule_parse = {
        type = "remap";
        inputs = [ "cleanuparr_stats" ];
        drop_on_abort = true;
        source = ''
          s = parse_json!(string!(.message))
          acc = []
          for_each(object(s.jobs.byType) ?? {}) -> |name, j| {
            last = parse_timestamp(j.lastRunAt, "%+") ?? null
            if last != null {
              acc = push(acc, {
                "key": name,
                "which": "last",
                "value": to_float(to_unix_timestamp(timestamp!(last))),
              })
            }
            next = parse_timestamp(j.nextRunAt, "%+") ?? null
            if next != null {
              acc = push(acc, {
                "key": name,
                "which": "next",
                "value": to_float(to_unix_timestamp(timestamp!(next))),
              })
            }
          }
          . = acc
        '';
      };
      cleanuparr_jobschedule_metric = {
        type = "log_to_metric";
        inputs = [ "cleanuparr_jobschedule_parse" ];
        metrics = [
          {
            type = "gauge";
            field = "value";
            name = "cleanuparr_job_run_timestamp_seconds";
            tags = {
              job = "{{key}}";
              which = "{{which}}";
            };
          }
        ];
      };

      # ── Connected service health ────────────────────────────────────────
      # Both arrays share a shape apart from responseTimeMs, which only the
      # download clients carry; the *arr entries publish no latency series
      # rather than a fake zero. `kind` keeps the two apart on a panel.
      cleanuparr_health_parse = {
        type = "remap";
        inputs = [ "cleanuparr_stats" ];
        drop_on_abort = true;
        source = ''
          s = parse_json!(string!(.message))
          acc = []
          for_each(array(s.health.downloadClients) ?? []) -> |_i, c| {
            healthy = 0.0
            if to_bool(c.isHealthy) ?? false {
              healthy = 1.0
            }
            acc = push(acc, {
              "key": to_string(c.name) ?? "unknown",
              "kind": "download_client",
              "healthy": healthy,
              "response": (to_float(c.responseTimeMs) ?? 0.0) / 1000.0,
              "has_response": 1.0,
            })
          }
          for_each(array(s.health.arrInstances) ?? []) -> |_i, a| {
            healthy = 0.0
            if to_bool(a.isHealthy) ?? false {
              healthy = 1.0
            }
            acc = push(acc, {
              "key": to_string(a.name) ?? "unknown",
              "kind": "arr",
              "healthy": healthy,
              "response": 0.0,
              "has_response": 0.0,
            })
          }
          . = acc
        '';
      };
      cleanuparr_health_metric = {
        type = "log_to_metric";
        inputs = [ "cleanuparr_health_parse" ];
        metrics = [
          {
            type = "gauge";
            field = "healthy";
            name = "cleanuparr_service_healthy";
            tags = {
              service = "{{key}}";
              kind = "{{kind}}";
            };
          }
        ];
      };
      cleanuparr_latency_filter = {
        type = "filter";
        inputs = [ "cleanuparr_health_parse" ];
        condition = ".has_response == 1.0";
      };
      cleanuparr_latency_metric = {
        type = "log_to_metric";
        inputs = [ "cleanuparr_latency_filter" ];
        metrics = [
          {
            type = "gauge";
            field = "response";
            name = "cleanuparr_client_response_seconds";
            tags.client = "{{key}}";
          }
        ];
      };
    }
    // builtins.foldl' (acc: m: acc // mapMetric m) { } maps;
  };
}
