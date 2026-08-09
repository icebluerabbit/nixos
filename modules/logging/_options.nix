# modules/logging/_options.nix
{
  lib,
  defaultHostType ? "host",
}:
{
  enable = lib.mkEnableOption "Vector logging agent";
  lokiAddress = lib.mkOption {
    type = lib.types.nullOr lib.types.str;
    default = null;
    description = "Loki push endpoint URL";
  };
  prometheusAddress = lib.mkOption {
    type = lib.types.nullOr lib.types.str;
    default = null;
    description = "Prometheus remote write endpoint URL";
  };
  tempoAddress = lib.mkOption {
    type = lib.types.nullOr lib.types.str;
    default = null;
    description = "Tempo remote write/push OTLP endpoint URL";
  };
  prometheusInputs = lib.mkOption {
    type = lib.types.listOf lib.types.str;
    default = [ ];
    description = "List of Vector source/transform IDs to push to the Prometheus sink";
  };
  hostType = lib.mkOption {
    type = lib.types.str;
    default = defaultHostType;
    description = "Classification of the metrics/logs source (host, container, or network)";
  };
  lokiInputs = lib.mkOption {
    type = lib.types.listOf lib.types.str;
    default = [ "journal" ];
    description = "List of Vector source/transform IDs to push to the Loki sink";
  };
  journalDropPatterns = lib.mkOption {
    type = lib.types.listOf lib.types.str;
    default = [ ];
    example = [ "Queue: \\d+ waiting" ];
    description = ''
      Regexes matched against a journal event's message. Matching events are dropped
      before the Loki sink — for chatty INFO lines that carry no signal and only cost
      ingest. When non-empty, `journal` in lokiInputs is swapped for the filtered
      stream; other inputs are untouched.
    '';
  };
  extraSettings = lib.mkOption {
    type = lib.types.attrs;
    default = { };
    description = "Extra raw Vector configurations (sources, transforms, sinks) to merge recursively";
  };
  environmentFile = lib.mkOption {
    type = lib.types.nullOr lib.types.path;
    default = null;
    description = "Path to an environment file loaded by the Vector systemd service";
  };
  validateConfig = lib.mkOption {
    type = lib.types.bool;
    default = true;
    description = "Whether to validate the Vector config at build time (disable when config uses runtime env vars)";
  };
}
