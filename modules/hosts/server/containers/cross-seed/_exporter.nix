/*
  Vector metrics for cross-seed.

  cross-seed exposes no /metrics and no stats API — `/api/status` answers with
  the literal string "OK", and the only other routes are /api/announce,
  /api/job, /api/ping and /api/webhook. So metrics come from two places:

    * the notification webhook. cross-seed already POSTs its search results to
      this sidecar (the `webhook_in` source in cross-seed.nix, which
      republishes them to MQTT for Home Assistant), and pushNotifier.js sends
      one event per searchee for successes *and* failures, carrying a
      structured `extra`: the injection result, every tracker hit and every
      decision that matched. Counting that involves no log text at all.
    * the journal, but only for what the webhook cannot report — log levels,
      searches skipped before they ever produced a result, and the scheduler's
      job cadence.

  Everything journal-derived goes through the single `cross_seed_parse`
  transform, because two things broke the previous version of this file:

    * winston colourises the level, so a raw line reads
      "2026-08-09 21:02:48 \e[32minfo\e[39m: [webhook] …". A regex for `warn:`
      can never match — the colon sits on the far side of the reset sequence —
      and one for `error:` matches only message *bodies* containing that text,
      which is why cross_seed_warnings_total never existed and
      cross_seed_errors_total counted the wrong thing. Strip SGR first.
    * `journal` is the whole container's journal. pam, systemd-tmpfiles and
      Vector's own logs are in it, so the unit filter has to come first or any
      regex here can match a foreign line.

  Note on liveness: an unreachable /api/ping produces no event rather than a 0,
  so cross_seed_up goes stale instead of dropping. `vector_build_info` is the
  heartbeat to alert on (see the prometheus sink comment in modules/logging).
*/
{ port }:
let
  base = "http://127.0.0.1:${toString port}";
in
{
  prometheusInputs = [
    "cross_seed_up_metric"
    "cross_seed_result_metric"
    "cross_seed_result_tracker_metric"
    "cross_seed_result_decision_metric"
    "cross_seed_line_metric"
    "cross_seed_search_metric"
    "cross_seed_match_metric"
    "cross_seed_injection_metric"
    "cross_seed_summary_metric"
    "cross_seed_summary_failure_metric"
    "cross_seed_announce_metric"
    "cross_seed_job_metric"
  ];

  extraSettings = {
    sources.cross_seed_ping = {
      type = "http_client";
      endpoint = "${base}/api/ping";
      scrape_interval_secs = 60;
      scrape_timeout_secs = 15;
    };

    transforms = {
      # ── Liveness ────────────────────────────────────────────────────────
      cross_seed_up_parse = {
        type = "remap";
        inputs = [ "cross_seed_ping" ];
        drop_on_abort = true;
        source = ''
          .up = if (string!(.message) == "OK") { 1 } else { 0 }
        '';
      };
      cross_seed_up_metric = {
        type = "log_to_metric";
        inputs = [ "cross_seed_up_parse" ];
        metrics = [
          {
            type = "gauge";
            field = "up";
            name = "cross_seed_up";
          }
        ];
      };

      # ── Results, from cross-seed's own notification webhook ──────────────
      # Body is cross-seed's JSON: { title, body, extra: { event, name,
      # trackers[], decisions[], result, source, searchee: {…} } }. `result` is
      # InjectionResult.SUCCESS / SaveResult.SAVED / InjectionResult.FAILURE.
      cross_seed_result_parse = {
        type = "remap";
        inputs = [ "webhook_in" ];
        drop_on_abort = true;
        source = ''
          n = parse_json!(string!(.message))
          extra = object(n.extra) ?? {}
          if (string(extra.event) ?? "") != "RESULTS" { abort }
          searchee = object(extra.searchee) ?? {}
          . = {
            "result": downcase(string(extra.result) ?? "unknown"),
            "source": string(searchee.source) ?? "unknown",
            "trackers": array(extra.trackers) ?? [],
            "decisions": array(extra.decisions) ?? [],
            "count": 1,
          }
        '';
      };
      cross_seed_result_metric = {
        type = "log_to_metric";
        inputs = [ "cross_seed_result_parse" ];
        metrics = [
          {
            type = "counter";
            field = "count";
            name = "cross_seed_results_total";
            tags = {
              result = "{{result}}";
              source = "{{source}}";
            };
          }
        ];
      };

      # One event per tracker / per decision: a single result can hit several
      # trackers, and a remap that assigns an array to `.` is how Vector fans
      # one event out into many (there is no unnest transform).
      cross_seed_result_tracker_expand = {
        type = "remap";
        inputs = [ "cross_seed_result_parse" ];
        source = ''
          result = string!(.result)
          . = map_values(array!(.trackers)) -> |t| {
            {
              "result": result,
              "tracker": to_string(t) ?? "unknown",
              "count": 1,
            }
          }
        '';
      };
      cross_seed_result_tracker_metric = {
        type = "log_to_metric";
        inputs = [ "cross_seed_result_tracker_expand" ];
        metrics = [
          {
            type = "counter";
            field = "count";
            name = "cross_seed_result_trackers_total";
            tags = {
              result = "{{result}}";
              tracker = "{{tracker}}";
            };
          }
        ];
      };
      cross_seed_result_decision_expand = {
        type = "remap";
        inputs = [ "cross_seed_result_parse" ];
        source = ''
          result = string!(.result)
          . = map_values(array!(.decisions)) -> |d| {
            {
              "result": result,
              "decision": to_string(d) ?? "unknown",
              "count": 1,
            }
          }
        '';
      };
      cross_seed_result_decision_metric = {
        type = "log_to_metric";
        inputs = [ "cross_seed_result_decision_expand" ];
        metrics = [
          {
            type = "counter";
            field = "count";
            name = "cross_seed_result_decisions_total";
            tags = {
              result = "{{result}}";
              decision = "{{decision}}";
            };
          }
        ];
      };

      # ── Journal: unit filter + ANSI strip + level/component split ────────
      cross_seed_parse = {
        type = "remap";
        inputs = [ "journal" ];
        drop_on_abort = true;
        source = ''
          if (string(._SYSTEMD_UNIT) ?? "") != "cross-seed.service" { abort }
          line = replace(string!(.message), r'\x1b\[[0-9;]*m', "")
          m = parse_regex(line, r'^\S+ \S+ (?P<level>\w+): \[(?P<component>[\w-]+)\] (?P<body>.*)') ?? null
          if m == null { abort }
          . = {
            "level": m.level,
            "component": m.component,
            "body": m.body,
            "count": 1,
          }
        '';
      };
      cross_seed_line_metric = {
        type = "log_to_metric";
        inputs = [ "cross_seed_parse" ];
        metrics = [
          {
            type = "counter";
            field = "count";
            name = "cross_seed_log_lines_total";
            tags = {
              level = "{{level}}";
              component = "{{component}}";
            };
          }
        ];
      };

      # ── Search decisions ────────────────────────────────────────────────
      cross_seed_search_parse = {
        type = "remap";
        inputs = [ "cross_seed_parse" ];
        drop_on_abort = true;
        source = ''
          body = string!(.body)
          if starts_with(body, "Did not search for") {
            .outcome = "skipped"
            # The tail is free text with numbers in it ("nonVideoSizeRatio
            # 1.000 > 0.02 fuzzySizeThreshold") and a release name earlier in
            # the line, so keep only the leading upstream identifier — nothing
            # unbounded may reach a Prometheus label. The greedy prefix anchors
            # on the *last* " - ", since release names contain dashes.
            r = parse_regex(body, r'.* - (?P<reason>[A-Za-z][A-Za-z_]*)') ?? null
            .reason = if r == null { "other" } else { r.reason }
          } else if contains(body, "Searching for") {
            .outcome = "searched"
            .reason = "none"
          } else if starts_with(body, "Received search request") {
            .outcome = "requested"
            .reason = "none"
          } else {
            abort
          }
        '';
      };
      cross_seed_search_metric = {
        type = "log_to_metric";
        inputs = [ "cross_seed_search_parse" ];
        metrics = [
          {
            type = "counter";
            field = "count";
            name = "cross_seed_search_decisions_total";
            tags = {
              outcome = "{{outcome}}";
              reason = "{{reason}}";
            };
          }
        ];
      };

      # ── Candidates returned per completed search ────────────────────────
      cross_seed_match_parse = {
        type = "remap";
        inputs = [ "cross_seed_parse" ];
        drop_on_abort = true;
        source = ''
          m = parse_regex(string!(.body), r'^Found (?P<n>\d+) torrents') ?? null
          if m == null { abort }
          .found = to_float(m.n) ?? 0.0
        '';
      };
      cross_seed_match_metric = {
        type = "log_to_metric";
        inputs = [ "cross_seed_match_parse" ];
        metrics = [
          {
            type = "counter";
            field = "found";
            name = "cross_seed_matches_found_total";
            increment_by_value = true;
          }
          {
            type = "counter";
            field = "count";
            name = "cross_seed_searches_completed_total";
          }
        ];
      };

      # ── Per-injection result ────────────────────────────────────────────
      # "Injected <path> - <RESULT>", results SUCCESS / ALREADY_EXISTS /
      # TORRENT_NOT_COMPLETE / FAILURE. The run summary below has no " - ", so
      # the two cannot collide.
      cross_seed_injection_parse = {
        type = "remap";
        inputs = [ "cross_seed_parse" ];
        drop_on_abort = true;
        source = ''
          m = parse_regex(string!(.body), r'^Injected .+ - (?P<result>[A-Z][A-Z_]+)') ?? null
          if m == null { abort }
          .result = m.result
        '';
      };
      cross_seed_injection_metric = {
        type = "log_to_metric";
        inputs = [ "cross_seed_injection_parse" ];
        metrics = [
          {
            type = "counter";
            field = "count";
            name = "cross_seed_injections_total";
            tags.result = "{{result}}";
          }
        ];
      };

      # ── Inject run summary ──────────────────────────────────────────────
      # "Injected <n>/<total> torrents[, <k> had <reason>]". The two _recent
      # gauge names are kept from the previous version of this file so their
      # history survives the rewrite.
      cross_seed_summary_parse = {
        type = "remap";
        inputs = [ "cross_seed_parse" ];
        drop_on_abort = true;
        source = ''
          m = parse_regex(string!(.body), r'^Injected (?P<inj>\d+)/(?P<tot>\d+) torrents') ?? null
          if m == null { abort }
          .injected_recent = to_float(m.inj) ?? 0.0
          .candidates_recent = to_float(m.tot) ?? 0.0
        '';
      };
      cross_seed_summary_metric = {
        type = "log_to_metric";
        inputs = [ "cross_seed_summary_parse" ];
        metrics = [
          {
            type = "gauge";
            field = "injected_recent";
            name = "cross_seed_injected_recent";
          }
          {
            type = "gauge";
            field = "candidates_recent";
            name = "cross_seed_candidates_recent";
          }
          {
            type = "counter";
            field = "injected_recent";
            name = "cross_seed_injected_total";
            increment_by_value = true;
          }
          {
            type = "counter";
            field = "candidates_recent";
            name = "cross_seed_inject_candidates_total";
            increment_by_value = true;
          }
        ];
      };
      cross_seed_summary_failure_parse = {
        type = "remap";
        inputs = [ "cross_seed_parse" ];
        drop_on_abort = true;
        source = ''
          m = parse_regex(
            string!(.body),
            r'^Injected \d+/\d+ torrents, (?P<n>\d+) had (?P<reason>[a-z]+(?: [a-z]+)*)'
          ) ?? null
          if m == null { abort }
          .failed = to_float(m.n) ?? 0.0
          .reason = replace(string!(m.reason), " ", "_")
        '';
      };
      cross_seed_summary_failure_metric = {
        type = "log_to_metric";
        inputs = [ "cross_seed_summary_failure_parse" ];
        metrics = [
          {
            type = "counter";
            field = "failed";
            name = "cross_seed_inject_failures_total";
            increment_by_value = true;
            tags.reason = "{{reason}}";
          }
        ];
      };

      # ── Announce handling ───────────────────────────────────────────────
      # "<verb> <release> from <tracker> (status: <code>)". Greedy prefix so a
      # release name containing " from " cannot steal the tracker capture.
      cross_seed_announce_parse = {
        type = "remap";
        inputs = [ "cross_seed_parse" ];
        drop_on_abort = true;
        source = ''
          body = string!(.body)
          m = parse_regex(body, r'.* from (?P<tracker>\S+) \(status: (?P<status>\d+)\)') ?? null
          if m == null { abort }
          .tracker = m.tracker
          .status = m.status
          .result = if starts_with(body, "Already exists") {
            "already_exists"
          } else if starts_with(body, "Injected") {
            "injected"
          } else if starts_with(body, "Saved") {
            "saved"
          } else {
            "other"
          }
        '';
      };
      cross_seed_announce_metric = {
        type = "log_to_metric";
        inputs = [ "cross_seed_announce_parse" ];
        metrics = [
          {
            type = "counter";
            field = "count";
            name = "cross_seed_announces_total";
            tags = {
              tracker = "{{tracker}}";
              status = "{{status}}";
              result = "{{result}}";
            };
          }
        ];
      };

      # ── Scheduler cadence ───────────────────────────────────────────────
      cross_seed_job_parse = {
        type = "remap";
        inputs = [ "cross_seed_parse" ];
        drop_on_abort = true;
        source = ''
          m = parse_regex(string!(.body), r'^starting job: (?P<job>\S+)') ?? null
          if m == null { abort }
          .job = m.job
        '';
      };
      cross_seed_job_metric = {
        type = "log_to_metric";
        inputs = [ "cross_seed_job_parse" ];
        metrics = [
          {
            type = "counter";
            field = "count";
            name = "cross_seed_jobs_started_total";
            tags.job = "{{job}}";
          }
        ];
      };
    };
  };
}
