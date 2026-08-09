# Vector log-derived Prometheus metrics for cross-seed.
# cross-seed has no /metrics endpoint, so we parse its journal (log level,
# per-injection results, and the run summary) into metrics via log_to_metric.
# Merged into the container's Vector sidecar by cross-seed.nix.
{
  prometheusInputs = [
    "cross_seed_errors_metric"
    "cross_seed_warnings_metric"
    "cross_seed_injections_metric"
    "cross_seed_summary_metric"
  ];

  extraSettings.transforms = {
    # "… error: …" level lines → error counter
    cross_seed_errors_filter = {
      type = "filter";
      inputs = [ "journal" ];
      condition = ''match(to_string(.message) ?? "", r'\berror:')'';
    };
    cross_seed_errors_metric = {
      type = "log_to_metric";
      inputs = [ "cross_seed_errors_filter" ];
      metrics = [
        {
          type = "counter";
          field = "message";
          name = "cross_seed_errors_total";
        }
      ];
    };

    # "… warn: …" level lines → warning counter
    cross_seed_warnings_filter = {
      type = "filter";
      inputs = [ "journal" ];
      condition = ''match(to_string(.message) ?? "", r'\bwarn:')'';
    };
    cross_seed_warnings_metric = {
      type = "log_to_metric";
      inputs = [ "cross_seed_warnings_filter" ];
      metrics = [
        {
          type = "counter";
          field = "message";
          name = "cross_seed_warnings_total";
        }
      ];
    };

    # "Injected <path> - <RESULT>" → per-injection counter by result
    # (INJECTED / ALREADY_EXISTS / SAVED / FAILURE / …).
    cross_seed_injection_parse = {
      type = "remap";
      inputs = [ "journal" ];
      source = ''
        m = parse_regex(to_string(.message) ?? "", r'Injected .+ - .*?(?P<result>[A-Z][A-Z_]+)') ?? null
        if m == null { abort }
        .result = m.result
      '';
    };
    cross_seed_injections_metric = {
      type = "log_to_metric";
      inputs = [ "cross_seed_injection_parse" ];
      metrics = [
        {
          type = "counter";
          field = "result";
          name = "cross_seed_injections_total";
          tags.result = "{{result}}";
        }
      ];
    };

    # "Injected <n>/<total> torrents" run summary → recent gauges
    cross_seed_summary_parse = {
      type = "remap";
      inputs = [ "journal" ];
      source = ''
        m = parse_regex(to_string(.message) ?? "", r'Injected (?P<inj>\d+)/(?P<tot>\d+) torrents') ?? null
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
      ];
    };
  };
}
