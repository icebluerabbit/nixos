# Vector log-derived Prometheus metrics for FlareSolverr.
# FlareSolverr has no native /metrics endpoint, so we parse its journal lines
# (request status, challenge outcome, solve latency, errors) into metrics via
# log_to_metric. Merged into the container's Vector sidecar by flaresolverr.nix.
{
  prometheusInputs = [
    "flaresolverr_requests_metric"
    "flaresolverr_challenge_metric"
    "flaresolverr_response_metric"
    "flaresolverr_errors_metric"
  ];

  extraSettings.transforms = {
    # "<ip> POST http://…/v1 200 OK" → requests by method + status
    flaresolverr_req_parse = {
      type = "remap";
      inputs = [ "journal" ];
      source = ''
        m = parse_regex(to_string(.message) ?? "", r'(?P<method>GET|POST) https?://\S+ (?P<status>\d{3}) ') ?? null
        if m == null { abort }
        .method = m.method
        .status = m.status
      '';
    };
    flaresolverr_requests_metric = {
      type = "log_to_metric";
      inputs = [ "flaresolverr_req_parse" ];
      metrics = [
        {
          type = "counter";
          field = "status";
          name = "flaresolverr_requests_total";
          tags = {
            method = "{{method}}";
            status = "{{status}}";
          };
        }
      ];
    };

    # "Challenge not detected!/detected/solved" → challenge outcomes
    flaresolverr_challenge_parse = {
      type = "remap";
      inputs = [ "journal" ];
      source = ''
        m = parse_regex(to_string(.message) ?? "", r'Challenge (?P<result>not_detected|not detected|detected|solved)') ?? null
        if m == null { abort }
        .result = m.result
      '';
    };
    flaresolverr_challenge_metric = {
      type = "log_to_metric";
      inputs = [ "flaresolverr_challenge_parse" ];
      metrics = [
        {
          type = "counter";
          field = "result";
          name = "flaresolverr_challenge_total";
          tags.result = "{{result}}";
        }
      ];
    };

    # "Response in 0.359 s" → solve latency gauge
    flaresolverr_response_parse = {
      type = "remap";
      inputs = [ "journal" ];
      source = ''
        m = parse_regex(to_string(.message) ?? "", r'Response in (?P<s>[0-9.]+) s') ?? null
        if m == null { abort }
        .response_seconds = to_float(m.s) ?? 0.0
      '';
    };
    flaresolverr_response_metric = {
      type = "log_to_metric";
      inputs = [ "flaresolverr_response_parse" ];
      metrics = [
        {
          type = "gauge";
          field = "response_seconds";
          name = "flaresolverr_response_seconds";
        }
      ];
    };

    # ERROR-level lines → error counter
    flaresolverr_errors_filter = {
      type = "filter";
      inputs = [ "journal" ];
      condition = ''match(to_string(.message) ?? "", r' ERROR ')'';
    };
    flaresolverr_errors_metric = {
      type = "log_to_metric";
      inputs = [ "flaresolverr_errors_filter" ];
      metrics = [
        {
          type = "counter";
          field = "message";
          name = "flaresolverr_errors_total";
        }
      ];
    };
  };
}
