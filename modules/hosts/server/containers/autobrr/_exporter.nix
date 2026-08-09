# autobrr metrics for the Vector sidecar.
#
# autobrr exposes no /metrics endpoint, so this polls two REST endpoints
# with X-API-Token = ${AUTOBRR_API_TOKEN} (from the sidecar's autobrr-env
# EnvironmentFile) and derives gauges via VRL + log_to_metric.
#
# /api/release/stats  — all-time push totals (approved / rejected / error / filtered)
# /api/irc            — per-network connected + healthy booleans, summed to counts
#
# The two *_metrics transforms must be listed in logging.prometheusInputs so
# they reach the prometheus sink (add_host_label tags them host="autobrr").
{ port }:
let
  auth = [ "\${AUTOBRR_API_TOKEN:-}" ];
  base = "http://127.0.0.1:${toString port}";
in
{
  sources = {
    autobrr_release_stats = {
      type = "http_client";
      endpoint = "${base}/api/release/stats";
      scrape_interval_secs = 60;
      scrape_timeout_secs = 30;
      headers."X-API-Token" = auth;
    };
    autobrr_irc = {
      type = "http_client";
      endpoint = "${base}/api/irc";
      scrape_interval_secs = 30;
      scrape_timeout_secs = 20;
      headers."X-API-Token" = auth;
    };
  };

  transforms = {
    autobrr_release_stats_parse = {
      type = "remap";
      inputs = [ "autobrr_release_stats" ];
      drop_on_abort = true;
      source = ''
        s = parse_json!(string!(.message))
        .push_approved = to_int(s.push_approved_count) ?? 0
        .push_rejected = to_int(s.push_rejected_count) ?? 0
        .push_errors   = to_int(s.push_error_count)    ?? 0
        .filtered      = to_int(s.filtered_count)      ?? 0
      '';
    };
    autobrr_release_stats_metrics = {
      type = "log_to_metric";
      inputs = [ "autobrr_release_stats_parse" ];
      metrics = [
        {
          type = "gauge";
          field = "push_approved";
          name = "autobrr_push_approved_total";
        }
        {
          type = "gauge";
          field = "push_rejected";
          name = "autobrr_push_rejected_total";
        }
        {
          type = "gauge";
          field = "push_errors";
          name = "autobrr_push_errors_total";
        }
        {
          type = "gauge";
          field = "filtered";
          name = "autobrr_filtered_total";
        }
      ];
    };

    autobrr_irc_parse = {
      type = "remap";
      inputs = [ "autobrr_irc" ];
      drop_on_abort = true;
      source = ''
        nets = array!(parse_json!(string!(.message)))
        .irc_connected = length(filter(nets) -> |_i, v| { to_bool(v.connected) ?? false })
        .irc_healthy   = length(filter(nets) -> |_i, v| { to_bool(v.healthy)   ?? false })
        .up = 1
      '';
    };
    autobrr_irc_metrics = {
      type = "log_to_metric";
      inputs = [ "autobrr_irc_parse" ];
      metrics = [
        {
          type = "gauge";
          field = "irc_connected";
          name = "autobrr_irc_connected";
        }
        {
          type = "gauge";
          field = "irc_healthy";
          name = "autobrr_irc_healthy";
        }
        {
          type = "gauge";
          field = "up";
          name = "autobrr_up";
        }
      ];
    };
  };
}
