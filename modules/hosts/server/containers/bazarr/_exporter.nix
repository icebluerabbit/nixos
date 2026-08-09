# Bazarr metrics for the Vector sidecar.
#
# No packaged Bazarr exporter exists (exportarr has no bazarr backend), and
# Bazarr has no native /metrics endpoint, so this polls the Bazarr API directly
# with Vector `http_client` sources (auth via X-API-KEY = ${BAZARR_API_KEY},
# interpolated from the sidecar's bazarr-env EnvironmentFile) and derives gauges
# with VRL + log_to_metric — the same pattern jellyfin/_exporter.nix uses.
#
# Bazarr logs each subtitle download at DEBUG only, so a journal-derived download
# counter would emit nothing at the default INFO level; instead the download
# activity comes from /api/history/stats (daily {date,count} series summed over
# the returned window).
#
# Returns a `logging.extraSettings` fragment (sources + transforms). The two
# `*_metrics` transforms must be listed in `logging.prometheusInputs` so they
# reach the prometheus sink (add_host_label tags them host="bazarr").
{ port }:
let
  # http_client headers are arrays of strings. `:-` default keeps Vector's
  # build-time `validate` happy (no env in the sandbox); the real value comes
  # from the sidecar's bazarr-env EnvironmentFile at runtime.
  auth = [ "\${BAZARR_API_KEY:-}" ];
  base = "http://127.0.0.1:${toString port}";
in
{
  sources = {
    # Badge counters: wanted subtitles, throttled providers, health issues,
    # Sonarr/Radarr SignalR connection state.
    bazarr_badges = {
      type = "http_client";
      endpoint = "${base}/api/badges";
      scrape_interval_secs = 60;
      scrape_timeout_secs = 30;
      headers."X-API-KEY" = auth;
    };
    # Daily download history (series + movies), summed over the window below.
    bazarr_history = {
      type = "http_client";
      endpoint = "${base}/api/history/stats";
      scrape_interval_secs = 300;
      scrape_timeout_secs = 30;
      headers."X-API-KEY" = auth;
    };
  };

  transforms = {
    bazarr_badges_parse = {
      type = "remap";
      inputs = [ "bazarr_badges" ];
      drop_on_abort = true;
      source = ''
        b = parse_json!(string!(.message))
        .wanted_episodes     = to_int(b.episodes)  ?? 0
        .wanted_movies       = to_int(b.movies)    ?? 0
        .providers_throttled = to_int(b.providers) ?? 0
        .health_issues       = to_int(b.status)    ?? 0
        .sonarr_up = if (to_string(b.sonarr_signalr) ?? "") == "LIVE" { 1 } else { 0 }
        .radarr_up = if (to_string(b.radarr_signalr) ?? "") == "LIVE" { 1 } else { 0 }
        .up = 1
      '';
    };
    bazarr_badges_metrics = {
      type = "log_to_metric";
      inputs = [ "bazarr_badges_parse" ];
      metrics = [
        {
          type = "gauge";
          field = "wanted_episodes";
          name = "bazarr_wanted_episodes";
        }
        {
          type = "gauge";
          field = "wanted_movies";
          name = "bazarr_wanted_movies";
        }
        {
          type = "gauge";
          field = "providers_throttled";
          name = "bazarr_providers_throttled";
        }
        {
          type = "gauge";
          field = "health_issues";
          name = "bazarr_health_issues";
        }
        {
          type = "gauge";
          field = "sonarr_up";
          name = "bazarr_sonarr_up";
        }
        {
          type = "gauge";
          field = "radarr_up";
          name = "bazarr_radarr_up";
        }
        {
          type = "gauge";
          field = "up";
          name = "bazarr_up";
        }
      ];
    };

    bazarr_history_parse = {
      type = "remap";
      inputs = [ "bazarr_history" ];
      drop_on_abort = true;
      source = ''
        h = parse_json!(string!(.message))
        se = 0
        for_each(array(h.series) ?? []) -> |_i, v| { se = se + (to_int(v.count) ?? 0) }
        mo = 0
        for_each(array(h.movies) ?? []) -> |_i, v| { mo = mo + (to_int(v.count) ?? 0) }
        .dl_series = se
        .dl_movies = mo
        .dl_total  = se + mo
      '';
    };
    bazarr_history_metrics = {
      type = "log_to_metric";
      inputs = [ "bazarr_history_parse" ];
      metrics = [
        {
          type = "gauge";
          field = "dl_series";
          name = "bazarr_downloads_series_window";
        }
        {
          type = "gauge";
          field = "dl_movies";
          name = "bazarr_downloads_movies_window";
        }
        {
          type = "gauge";
          field = "dl_total";
          name = "bazarr_downloads_total_window";
        }
      ];
    };
  };
}
