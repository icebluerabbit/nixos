/*
  cross-seed.

  cross-seed publishes no metrics of its own, so everything here comes from the
  Vector sidecar (../../cross-seed/_exporter.nix): the result counters are
  derived from cross-seed's notification webhook — structured JSON, exact — and
  the level/skip/scheduler series from its journal.

  Two consequences worth knowing when reading these panels:

    * `cross_seed_results_total` only counts searchees that produced a notable
      outcome (an injection, a save, or a failure). A quiet period means
      nothing matched, not that nothing ran — `cross_seed_search_decisions_total`
      is the panel that shows whether cross-seed is actually working.
    * Injections are rare by design. Most announces are already held, and most
      searches match nothing; the failure-reason and skip-reason breakdowns are
      where the interesting signal lives, not the injection count.
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
  title = "cross-seed";
  uid = "cross-seed";
  description = "cross-seed: search decisions, why candidates are skipped, injection outcomes per tracker, and scheduler cadence. All series are Vector-derived — cross-seed exposes no metrics endpoint.";
  editable = true;
  graphTooltip = 1;
  schemaVersion = 40;
  refresh = "1m";
  timezone = "browser";
  tags = [
    "media"
    "download"
    "cross-seed"
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
      query = "error,warn,info,verbose,debug";
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
      title = "Health";
      y = 0;
    })
    (stat {
      id = 1;
      title = "Up";
      x = 0;
      y = 1;
      w = 3;
      expr = "cross_seed_up";
      colorMode = "background";
      textMode = "value_and_name";
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
      description = "cross-seed's /api/ping. Goes stale rather than to 0 if the daemon stops answering — an unreachable scrape emits no sample at all.";
    })
    (stat {
      id = 2;
      title = "Errors";
      x = 3;
      y = 1;
      w = 3;
      expr = ''sum(increase(cross_seed_log_lines_total{level="error"}[$__range]))'';
      decimals = 0;
      colorMode = "background";
      thresholds = redAbove 1;
    })
    (stat {
      id = 3;
      title = "Warnings";
      x = 6;
      y = 1;
      w = 3;
      expr = ''sum(increase(cross_seed_log_lines_total{level="warn"}[$__range]))'';
      decimals = 0;
      colorMode = "background";
      thresholds = redAbove 25;
    })
    (stat {
      id = 4;
      title = "Searches";
      x = 9;
      y = 1;
      w = 3;
      expr = "sum(increase(cross_seed_searches_completed_total[$__range]))";
      decimals = 0;
      graphMode = "area";
    })
    (stat {
      id = 5;
      title = "Candidates found";
      x = 12;
      y = 1;
      w = 3;
      expr = "sum(increase(cross_seed_matches_found_total[$__range]))";
      decimals = 0;
      graphMode = "area";
      description = "Torrents returned by the indexers across all completed searches. Most are then rejected on comparison — this is the top of the funnel, not a match count.";
    })
    (stat {
      id = 6;
      title = "Injected";
      x = 15;
      y = 1;
      w = 3;
      expr = "sum(increase(cross_seed_injected_total[$__range]))";
      decimals = 0;
      graphMode = "area";
    })
    (stat {
      id = 7;
      title = "Inject success";
      x = 18;
      y = 1;
      w = 3;
      expr = "100 * sum(increase(cross_seed_injected_total[$__range])) / clamp_min(sum(increase(cross_seed_inject_candidates_total[$__range])), 1)";
      unit = "percent";
      decimals = 1;
      description = "Injected as a share of the torrents each inject run considered. Denominator is clamped to 1, so an idle window reads 0% rather than dividing by zero.";
    })
    (stat {
      id = 8;
      title = "Failures";
      x = 21;
      y = 1;
      w = 3;
      expr = ''sum(increase(cross_seed_results_total{result="failure"}[$__range]))'';
      decimals = 0;
      colorMode = "background";
      thresholds = redAbove 1;
    })

    (row {
      id = 101;
      title = "Search funnel";
      y = 5;
    })
    (ts {
      id = 10;
      title = "Search decisions";
      x = 0;
      y = 6;
      w = 12;
      targets = [
        (target ''sum(increase(cross_seed_search_decisions_total{outcome="requested"}[$__interval]))''
          "requested"
          "A"
        )
        (target ''sum(increase(cross_seed_search_decisions_total{outcome="searched"}[$__interval]))''
          "searched"
          "B"
        )
        (target ''sum(increase(cross_seed_search_decisions_total{outcome="skipped"}[$__interval]))''
          "skipped"
          "C"
        )
        (target "sum(increase(cross_seed_matches_found_total[$__interval]))" "candidates found" "D")
      ];
      drawStyle = "bars";
      fillOpacity = 50;
      decimals = 0;
      description = "The whole funnel: announces received, those actually searched, those rejected before searching, and the candidates the searches returned.";
    })
    (bar {
      id = 11;
      title = "Why searches were skipped";
      x = 12;
      y = 6;
      w = 12;
      expr = ''topk(10, sum by (reason) (increase(cross_seed_search_decisions_total{outcome="skipped"}[$__range])))'';
      legendFormat = "{{reason}}";
      description = "Leading identifier of cross-seed's rejection reason (nonVideoSizeRatio, …). Only the identifier is kept as a label — the rest of the line carries release names and would be unbounded cardinality.";
    })

    (row {
      id = 102;
      title = "Injection";
      y = 14;
    })
    (ts {
      id = 20;
      title = "Injected vs considered";
      x = 0;
      y = 15;
      w = 12;
      targets = [
        (target "sum(increase(cross_seed_inject_candidates_total[$__interval]))" "considered" "A")
        (target "sum(increase(cross_seed_injected_total[$__interval]))" "injected" "B")
      ];
      drawStyle = "bars";
      fillOpacity = 50;
      decimals = 0;
    })
    (bar {
      id = 21;
      title = "Why injections did not happen";
      x = 12;
      y = 15;
      w = 6;
      expr = "topk(10, sum by (reason) (increase(cross_seed_inject_failures_total[$__range])))";
      legendFormat = "{{reason}}";
      description = "Tail of the inject run summary: no_matches, incomplete_sources, …";
    })
    (bar {
      id = 22;
      title = "Injection results";
      x = 18;
      y = 15;
      w = 6;
      expr = "sum by (result) (increase(cross_seed_injections_total[$__range]))";
      legendFormat = "{{result}}";
      description = "Per-torrent outcome: SUCCESS, ALREADY_EXISTS, TORRENT_NOT_COMPLETE, FAILURE.";
    })

    (row {
      id = 103;
      title = "Trackers";
      y = 23;
    })
    (bar {
      id = 30;
      title = "Results by tracker";
      x = 0;
      y = 24;
      w = 8;
      expr = "topk(15, sum by (tracker, result) (increase(cross_seed_result_trackers_total[$__range])))";
      legendFormat = "{{tracker}} — {{result}}";
      description = "From cross-seed's notification webhook. One result can span several trackers, so this sums higher than the result count.";
    })
    (bar {
      id = 31;
      title = "Matches by decision";
      x = 8;
      y = 24;
      w = 8;
      expr = "topk(10, sum by (decision) (increase(cross_seed_result_decisions_total[$__range])))";
      legendFormat = "{{decision}}";
      description = "Which comparison accepted the match — MATCH, MATCH_SIZE_ONLY, MATCH_PARTIAL.";
    })
    (ts {
      id = 32;
      title = "Announce handling";
      x = 16;
      y = 24;
      w = 8;
      targets = [
        (target "sum by (result) (increase(cross_seed_announces_total[$__interval]))" "{{result}}" "A")
      ];
      drawStyle = "bars";
      fillOpacity = 50;
      decimals = 0;
      description = "autobrr's announce POSTs, by what cross-seed did with them.";
    })

    (row {
      id = 104;
      title = "Scheduler and logs";
      y = 32;
    })
    (ts {
      id = 40;
      title = "Scheduled jobs started";
      x = 0;
      y = 33;
      w = 12;
      targets = [
        (target "sum by (job) (increase(cross_seed_jobs_started_total[$__interval]))" "{{job}}" "A")
      ];
      drawStyle = "bars";
      fillOpacity = 50;
      decimals = 0;
      description = "search runs daily, rss every 30m, inject on each webhook. A flat line for `search` means the daily sweep has stopped.";
    })
    (ts {
      id = 41;
      title = "Log volume by level";
      x = 12;
      y = 33;
      w = 12;
      targets = [
        (target ''sum by (level) (rate(cross_seed_log_lines_total{level=~"$level"}[$__rate_interval]))''
          "{{level}}"
          "A"
        )
      ];
      unit = "cps";
      description = "Levels are parsed after stripping winston's colour codes; before that the level and its colon were separated by an escape sequence and never matched.";
    })
    (logs {
      id = 50;
      title = "cross-seed log";
      x = 0;
      y = 41;
      expr = ''{host="cross-seed"} | json | _SYSTEMD_UNIT = "cross-seed.service" | line_format "{{.message}}"'';
      description = "Filtered to the cross-seed unit — the container journal also carries pam, systemd-tmpfiles and Vector's own output.";
    })
  ];
}
