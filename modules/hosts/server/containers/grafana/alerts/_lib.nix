# Rule builders. A Grafana unified-alerting rule is mostly ceremony: an A query
# against a real datasource, a B `__expr__` threshold that turns A into a boolean,
# and the same six model fields every time. These builders take only the parts that
# differ, so a rule file reads as its query + its threshold + what to say when it fires.
#
# datasourceUid is the real pinned uid (see ../datasources/) — alert rules resolve
# datasources by uid, not by name, and a name-ref fails eval with "data source not found".
#
# Required on purpose, never defaulted: `for`, `severity` and `noDataState`. They are
# the knobs that decide whether a rule is signal or spam, and a default would hide
# them from the rule file. `noDataState` in particular has no safe default — it is the
# answer to "what does silence mean for THIS query?", and the answer differs per rule:
#
#   OK        the query returns an empty vector when healthy (a threshold over a
#             gauge that only exists while the fault exists, e.g. absent_over_time).
#   Alerting  silence is itself the fault — the exporter that would answer this
#             query has stopped reporting (e.g. caddy upstream health: NoData means
#             Caddy is down, which is worse than an unhealthy upstream).
#
# Rules carry `severity` only. Room routing is the notification layer's business
# (HA maps the MQTT topic → Matrix room); a rule never names a room.
_:
let
  prometheusUid = "PBFA97CFB590B2093";
  lokiUid = "P8E80F9AEF21F6940";

  mkRule =
    {
      uid,
      title,
      expr,
      datasourceUid,
      summary,
      description,
      for,
      severity,
      noDataState,
      # What the alert is *about*: the subsystem, not the destination. The notification
      # layer maps a category to a Matrix room (several categories share one room), so a
      # rule states its subject and stays ignorant of where that gets said. Required, so
      # a new rule can't quietly inherit somebody else's routing.
      category,
      # Fire when A <op> value. `gt`/`lt` for ratios and counts; a boolean gauge
      # that reads `== 0` is expressed as `lt` 1.
      op,
      value,
      # How far back the A query may look. Instant queries only need a window wide
      # enough to cover the range selector inside the expression.
      lookback ? 600,
      # Keep firing this long after the condition clears — swallows flapping on
      # rules whose input is bursty (latency, error rates) without weakening `for`.
      keepFiringFor ? null,
      # How many evaluations a vanished series must stay missing before its alert
      # instance resolves. Matters for the per-host rules: when a container dies its
      # series disappears, and the default (1) can resolve the alert the moment the
      # thing it was alerting about goes fully dark.
      missingSeriesEvalsToResolve ? null,
      queryType ? null,
      isPaused ? false,
      labels ? { },
      annotations ? { },
    }:
    {
      inherit
        uid
        title
        for
        noDataState
        isPaused
        ;
      condition = "B";
      labels = {
        inherit severity category;
      }
      // labels;
      annotations = {
        inherit summary description;
      }
      // annotations;
      execErrState = "Error";
      data = [
        {
          refId = "A";
          relativeTimeRange = {
            from = lookback;
            to = 0;
          };
          inherit datasourceUid;
          model = {
            editorMode = "code";
            inherit expr;
            instant = true;
            intervalMs = 1000;
            maxDataPoints = 43200;
            refId = "A";
          }
          // (if queryType == null then { } else { inherit queryType; });
        }
        {
          refId = "B";
          relativeTimeRange = {
            from = 0;
            to = 0;
          };
          datasourceUid = "__expr__";
          model = {
            type = "threshold";
            expression = "A";
            conditions = [
              {
                evaluator = {
                  type = op;
                  params = [ value ];
                };
              }
            ];
            refId = "B";
          };
        }
      ];
    }
    // (if keepFiringFor == null then { } else { keep_firing_for = keepFiringFor; })
    // (
      if missingSeriesEvalsToResolve == null then
        { }
      else
        { missing_series_evals_to_resolve = missingSeriesEvalsToResolve; }
    );
in
{
  promRule = args: mkRule (args // { datasourceUid = prometheusUid; });

  # Loki rules must be instant metric queries (count_over_time / sum), never log
  # queries — a log-line result carries no value for the threshold to evaluate.
  lokiRule =
    args:
    mkRule (
      args
      // {
        datasourceUid = lokiUid;
        queryType = "instant";
      }
    );
}
