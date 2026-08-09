# modules/logging/_vector.nix
{
  config,
  lib,
  ...
}:
let
  cfg = config.logging.vector;
  dropJournal = cfg.lokiAddress != null && cfg.journalDropPatterns != [ ];
  # VRL has no multi-pattern helper: OR one match() per pattern, keep the event
  # only when none hit. Patterns are raw regexes (r'...'), so escaping stays theirs.
  journalKeep = lib.concatMapStringsSep " && " (p: "!match(msg, r'${p}')") cfg.journalDropPatterns;
in
{
  options.logging.vector = import ./_options.nix { inherit lib; };

  config = lib.mkIf cfg.enable {
    services.vector = {
      enable = true;
      journaldAccess = true;
      inherit (cfg) validateConfig;
      settings = lib.recursiveUpdate {
        sources = {
          vector_metrics = {
            type = "internal_metrics";
            scrape_interval_secs = 60;
          };
        }
        // lib.optionalAttrs (cfg.lokiAddress != null) {
          journal = {
            type = "journald";
          };
        }
        // lib.optionalAttrs (cfg.tempoAddress != null) {
          otlp_traces = {
            type = "opentelemetry";
            grpc.address = "127.0.0.1:4317";
            http.address = "127.0.0.1:4318";
          };
        };
        sinks = {
          loki = lib.mkIf (cfg.lokiAddress != null) {
            type = "loki";
            inputs = map (i: if dropJournal && i == "journal" then "journal_kept" else i) cfg.lokiInputs;
            endpoint = lib.removeSuffix "/loki/api/v1/push" cfg.lokiAddress;
            encoding.codec = "json";
            # Newer vector's confinement guard rejects a label value that is a bare
            # `{{ field }}` template with no literal prefix. `host` is a bounded,
            # trusted value (few hosts), so opt out rather than prefix-polluting the
            # label. Without this: Sink "loki" ... "no literal string prefix".
            dangerously_allow_unconfined_template_resolution = true;
            labels = {
              host = "{{ host }}";
              host_type = cfg.hostType;
            };
            batch.timeout_secs = 10;
            buffer = {
              type = "memory";
              max_events = 1000;
              when_full = "drop_newest";
            };
          };
          # Gated on the address alone, not on prometheusInputs. `vector_metrics`
          # (internal_metrics) is an unconditional source, so every target that names a
          # Prometheus endpoint ships its own sidecar metrics even when it has nothing
          # else to scrape — and `vector_build_info` is then a heartbeat for every target.
          # That matters because Prometheus here is fed by remote_write, not scraping:
          # there is no `up` series, so a container that publishes nothing publishes no
          # evidence that it is alive, and no liveness alert can be written for it.
          # Requiring prometheusInputs left 18 containers (zigbee2mqtt, lidarr, readarr,
          # tuwunel, lldap, pi-hole, kiwix, tempo, …) invisible to monitoring entirely.
          prometheus = lib.mkIf (cfg.prometheusAddress != null) {
            type = "prometheus_remote_write";
            endpoint = cfg.prometheusAddress;
            inputs = [ "add_host_label" ];
            healthcheck.enabled = false;
            batch.timeout_secs = 30;
            request.concurrency = 1;
            buffer = {
              type = "memory";
              max_events = 1000;
              when_full = "drop_newest";
            };
          };
          tempo = lib.mkIf (cfg.tempoAddress != null) {
            type = "opentelemetry";
            inputs = [ "otlp_traces.traces" ];
            protocol = {
              type = "http";
              uri = cfg.tempoAddress;
              encoding.codec = "otlp";
            };
          };
        };
        transforms = {
          add_host_label = lib.mkIf (cfg.prometheusAddress != null) {
            type = "remap";
            inputs = [ "vector_metrics" ] ++ cfg.prometheusInputs;
            source = ''
              .tags.host = "${config.networking.hostName}"
              .tags.host_type = "${cfg.hostType}"
            '';
          };
          # Journal noise gate — see logging.vector.journalDropPatterns. Only the
          # Loki path is filtered; journal-derived metrics/notify transforms keep
          # reading the raw `journal` source.
          journal_kept = lib.mkIf dropJournal {
            type = "filter";
            inputs = [ "journal" ];
            condition = ''
              msg = to_string(.message) ?? ""
              ${journalKeep}
            '';
          };
        };
      } cfg.extraSettings;
    };

    systemd.services.vector.serviceConfig.EnvironmentFile = lib.mkIf (
      cfg.environmentFile != null
    ) cfg.environmentFile;

    # Vector 0.57 gates env-var interpolation behind this flag (was default-on
    # pre-0.57). Sidecars inject scrape auth via ${VAR} from EnvironmentFile, so
    # without it every authed scrape ships the literal "${VAR}" and 401s.
    systemd.services.vector.environment.VECTOR_DANGEROUSLY_ALLOW_ENV_VAR_INTERPOLATION = "true";
  };
}
