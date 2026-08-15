---
name: homelab-observability
description: Investigate or tune anything on this homelab using measured evidence — metrics, logs and traces via the Grafana stack (Loki / Prometheus / Tempo) plus the sys-mcp observability tools. Use for "why is X slow", "should I change setting Y", "find errors in service Z", query logs/metrics, correlate a runtime issue, or judge whether a value is healthy. Every container, host and network device is instrumented, and that telemetry — not config comments, not prior reasoning — is the authority for any tuning or debugging decision. Also load homelab-topology to resolve targets and nixos-vector-logging when shipping itself is misconfigured.
---

# Homelab Observability

Every container and host ships logs, metrics and traces via a Vector sidecar
to Loki + Prometheus + Tempo. ~200k series across ~50 reporting entities,
~3000 distinct metric names, 70+ provisioned alert rules, 38 dashboards.

**The telemetry is the authority.** Not comments, not README claims, not what
a previous session concluded. If a decision can be measured, measure it.

## Doctrine

1. **A config comment is a hypothesis, never evidence.** Comments record what
   someone believed when they wrote them. Verify before repeating one. A
   comment in `_bittorrent.nix` asserted a "~30 MB/s BDP wall"; measurement
   showed 110 MB/s peaks. It had been wrong for months and was steering
   tuning decisions the whole time.
2. **No claim without a query, a window and a number.** "Should help",
   "seems high", "probably fine" are not conclusions. Quote the metric, the
   time range, and the value.
3. **Check whether "bad" is already defined** before calling any value high
   or low. 70+ alert rules carry committed thresholds. `get_metric` returns
   the governing rule and its exact number — use it instead of inventing one.
4. **Say when a judgement is yours.** If a metric has no alert governing it,
   any threshold you apply is your invention. State that plainly.
5. **Report what the data shows, including when it refutes the plan.** A
   measurement that kills the proposed change is the most valuable result
   available, not a failure.

## Orient first — sys-mcp tools

Start here. These answer "what evidence exists" before you go looking for it.

```
describe_observability <host|container>   # families, logs, alerts, dashboards
get_metric <name>                         # current value + governing threshold
list_metric_families [host]               # what is instrumented
list_alert_thresholds [category] [metric] # the agreed definitions of bad
check_signal_coverage                     # gaps: what is NOT measured
```

`get_metric` is the workhorse. One call returns the live value, the alert
rule governing it with its exact threshold and duration, and the dashboard
aspect charting it — which is where panel `description` fields define what
the metric actually means.

Then use the `grafana` MCP server to run the real queries.

## The label backbone

Prometheus and Loki carry the same `host` and `host_type` labels, so a host
name moves between PromQL and LogQL unchanged. `host_type` is `container`,
`host`, or `network`.

Two traps, both confirmed live:

- **`service_name` is unpopulated.** Its only value estate-wide is
  `unknown_service`. Selecting on it matches nothing. Use `host`.
- **The join is not symmetric.** UniFi gear ships syslog to Loki under its
  device name (`UNAS-Pro`, `USW-Pro-HD-24-PoE`, …) while its metrics arrive
  via the unpoller exporter under a different host entirely. Joining those on
  `host` returns an empty result, not an error. `check_signal_coverage` lists
  the current mismatches.

An empty result never means healthy. It means empty. Distinguish "no series"
from "series reading zero" before concluding anything.

## Query technique

**Always pull avg, p95 and max together.** Averages hide binding behaviour.
`qbt_peers_uploading` averaged 62 against a cap of 100 — which reads slack —
while its max was 102, pinned at the ceiling with 224 peers queued behind it.

```promql
avg_over_time(m[7d])  quantile_over_time(0.95, m[7d])  max_over_time(m[7d])
```

**Conditional slicing** — compare a metric's behaviour under a condition on a
_different_ metric. This is how you test whether X actually drives Y instead
of eyeballing two lines on a dashboard:

```promql
avg_over_time((qbt_peers_uploading and on()
  (rate(node_pressure_io_stalled_seconds_total{host="server"}[5m]) > 0.2))[7d:5m])
```

Run it again with the condition inverted (`< 0.05`) and compare. A weak split
is evidence _against_ the causal story — that query returned 66 vs 52, which
is what killed the theory that upload slots were driving the IO stalls.

**Duty cycle** — what fraction of the time a condition holds:

```promql
avg_over_time((metric >= bool threshold)[7d:5m])
```

**Cross-entity joins need `on()`.** Metrics from different hosts carry
different `host` values, so an unqualified `and` matches nothing.

**`A or B` short-circuits.** If A returns any series, B never evaluates.
Combining two probes with `or` to save a round trip silently returns only the
first. Issue them separately.

## Investigation workflow

1. `describe_observability <target>` — what signal exists at all.
2. `get_metric` on the metrics that matter — current values and the
   thresholds already agreed for them.
3. Form a hypothesis and state it explicitly.
4. Query the `grafana` MCP server over an explicit window. Pull avg/p95/max,
   not just current.
5. Try to **refute** the hypothesis with conditional slicing. Correlation
   under one condition proves little; a clean split under the inverse is
   what makes it real.
6. Cross-reference: container journal (`container_journal`), host PSI
   (`node_pressure_*`), NFS (`node_nfs_*`), the alert history.
7. Report numbers with their windows. Name what stayed unverified.

## Logs (LogQL)

```
query_loki_logs '{host="radarr"} |~ "(?i)error|panic|fatal"'
query_loki_patterns / query_loki_stats / find_error_pattern_logs
```

Selector `{host="…"}` (**not** `service`), `|=` / `!=` line filters, `| json`
/ `| logfmt` parsers, rate via `count_over_time({…}[5m])`.

## Guards

- Read and analysis only. Creating dashboards, annotations, alert rules or
  incidents mutates Grafana — only when explicitly asked, and say so first.
- Always pass an explicit time range. Default windows miss the event.
- Grafana, Prometheus and Loki are themselves containers. If a datasource is
  unreachable the sys-mcp tools degrade to static config and say so — that
  says nothing about the target's health. Check the datasource container via
  `container_status` (see `nixos-container-debug`).
- Tuning changes to container configs still need turn-by-turn approval, and
  `switch`/`apply` is always handed to the user.
