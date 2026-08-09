# Vector logging extras for soularr. soularr exposes no /metrics endpoint (bare script +
# flask UI), so its counters are derived from journal lines via log_to_metric — same idea
# as lidarr's RSS/warn metrics. All series are absent until the first run emits the line.
{
  prometheusInputs = [
    "soularr_event_metrics"
    "soularr_grab_metrics"
    "soularr_failedgrab_metrics"
  ];
  extraSettings = {
    transforms = {
      # Classify each journal line into a single event kind; drop everything else. One
      # tagged counter (soularr_events_total{event=...}) is cheaper than a filter+counter
      # pair per line and keeps the metric surface small.
      soularr_events = {
        type = "remap";
        inputs = [ "journal" ];
        source = ''
          # journald .message carries soularr's log prefix ("[INFO|soularr|Lxx] <ts>: <text>"),
          # so match on `contains`, not `starts_with`. Order matters: the "Search failed"/
          # "Download failed" checks must not be shadowed by the plain "Searching"/"download" ones.
          msg = to_string(.message) ?? ""
          .event = if contains(msg, "SUCCESSFUL MATCH") {
            "match"
          } else if contains(msg, "Search failed for Album:") {
            "search_failed"
          } else if contains(msg, "Searching for album:") {
            "search"
          } else if contains(msg, "Completed download of Album:") {
            "download_completed"
          } else if contains(msg, "Starting Lidarr import for:") {
            "import_attempt"
          } else if contains(msg, "Download failed for Album:") {
            "download_failed"
          } else if contains(msg, "Added to failed import denylist:") {
            "denylist"
          } else {
            null
          }
          if is_null(.event) {
            abort
          }
        '';
      };
      soularr_event_metrics = {
        type = "log_to_metric";
        inputs = [ "soularr_events" ];
        metrics = [
          {
            type = "counter";
            field = "event";
            name = "soularr_events_total";
            tags.event = "{{event}}";
          }
        ];
      };

      # Per-run gauges parsed from the end-of-run summary lines.
      soularr_grab_parse = {
        type = "remap";
        inputs = [ "journal" ];
        source = ''
          msg = to_string(.message) ?? ""
          m = parse_regex(msg, r'Total Downloads added: (?P<n>\d+)') ?? null
          if is_null(m) {
            abort
          }
          .downloads_added = to_int!(m.n)
        '';
      };
      soularr_grab_metrics = {
        type = "log_to_metric";
        inputs = [ "soularr_grab_parse" ];
        metrics = [
          {
            type = "gauge";
            field = "downloads_added";
            name = "soularr_downloads_added";
          }
        ];
      };

      soularr_failedgrab_parse = {
        type = "remap";
        inputs = [ "journal" ];
        source = ''
          msg = to_string(.message) ?? ""
          m = parse_regex(msg, r'Failed to grab: (?P<n>\d+)') ?? null
          if is_null(m) {
            abort
          }
          .failed_grab = to_int!(m.n)
        '';
      };
      soularr_failedgrab_metrics = {
        type = "log_to_metric";
        inputs = [ "soularr_failedgrab_parse" ];
        metrics = [
          {
            type = "gauge";
            field = "failed_grab";
            name = "soularr_failed_grab";
          }
        ];
      };
    };
  };
}
