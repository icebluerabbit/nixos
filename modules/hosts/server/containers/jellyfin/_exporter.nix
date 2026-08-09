# Jellyfin metrics for the Vector sidecar.
#
# No packaged Jellyfin exporter exists, so this polls the Jellyfin API directly
# with Vector `http_client` sources (auth via X-Emby-Token = ${JELLYFIN_API_KEY},
# interpolated from the sidecar's jellyfin-env EnvironmentFile) and derives gauges
# with VRL + log_to_metric — the same custom-metrics pattern the *arr containers use.
#
# Returns a `logging.extraSettings` fragment (sources + transforms). The three
# `*_metrics` transforms must be listed in `logging.prometheusInputs` so they reach
# the prometheus sink (add_host_label tags them host="jellyfin").
{ port }:
let
  # http_client headers are arrays of strings. `:-` default keeps Vector's
  # build-time `validate` happy (no env in the sandbox); the real value comes
  # from the sidecar's jellyfin-env EnvironmentFile at runtime.
  auth = [ "\${JELLYFIN_API_KEY:-}" ];
  base = "http://127.0.0.1:${toString port}";
in
{
  sources = {
    # Library item totals.
    jellyfin_counts = {
      type = "http_client";
      endpoint = "${base}/Items/Counts";
      scrape_interval_secs = 60;
      scrape_timeout_secs = 30;
      headers."X-Emby-Token" = auth;
    };
    # Active playback sessions (streams / transcodes).
    jellyfin_sessions = {
      type = "http_client";
      endpoint = "${base}/Sessions?activeWithinSeconds=960";
      scrape_interval_secs = 30;
      scrape_timeout_secs = 20;
      headers."X-Emby-Token" = auth;
    };
    # Liveness — any successful scrape emits jellyfin_up=1.
    jellyfin_up = {
      type = "http_client";
      endpoint = "${base}/System/Info";
      scrape_interval_secs = 60;
      scrape_timeout_secs = 20;
      headers."X-Emby-Token" = auth;
    };
  };

  transforms = {
    jellyfin_counts_parse = {
      type = "remap";
      inputs = [ "jellyfin_counts" ];
      drop_on_abort = true;
      source = ''
        c = parse_json!(string!(.message))
        .movies   = to_int(c.MovieCount)   ?? 0
        .series   = to_int(c.SeriesCount)  ?? 0
        .episodes = to_int(c.EpisodeCount) ?? 0
        .albums   = to_int(c.AlbumCount)   ?? 0
        .songs    = to_int(c.SongCount)    ?? 0
      '';
    };
    jellyfin_counts_metrics = {
      type = "log_to_metric";
      inputs = [ "jellyfin_counts_parse" ];
      metrics = [
        {
          type = "gauge";
          field = "movies";
          name = "jellyfin_library_movies";
        }
        {
          type = "gauge";
          field = "series";
          name = "jellyfin_library_series";
        }
        {
          type = "gauge";
          field = "episodes";
          name = "jellyfin_library_episodes";
        }
        {
          type = "gauge";
          field = "albums";
          name = "jellyfin_library_albums";
        }
        {
          type = "gauge";
          field = "songs";
          name = "jellyfin_library_songs";
        }
      ];
    };

    jellyfin_sessions_parse = {
      type = "remap";
      inputs = [ "jellyfin_sessions" ];
      drop_on_abort = true;
      source = ''
        s = array!(parse_json!(string!(.message)))
        .sessions   = length(s)
        .streams    = length(filter(s) -> |_i, v| { exists(v.NowPlayingItem) })
        .transcodes = length(filter(s) -> |_i, v| { exists(v.TranscodingInfo) })
      '';
    };
    jellyfin_sessions_metrics = {
      type = "log_to_metric";
      inputs = [ "jellyfin_sessions_parse" ];
      metrics = [
        {
          type = "gauge";
          field = "sessions";
          name = "jellyfin_active_sessions";
        }
        {
          type = "gauge";
          field = "streams";
          name = "jellyfin_active_streams";
        }
        {
          type = "gauge";
          field = "transcodes";
          name = "jellyfin_transcodes";
        }
      ];
    };

    jellyfin_up_parse = {
      type = "remap";
      inputs = [ "jellyfin_up" ];
      drop_on_abort = true;
      source = ''
        . = parse_json!(string!(.message))
        .up = 1
      '';
    };
    jellyfin_up_metrics = {
      type = "log_to_metric";
      inputs = [ "jellyfin_up_parse" ];
      metrics = [
        {
          type = "gauge";
          field = "up";
          name = "jellyfin_up";
        }
      ];
    };
  };
}
