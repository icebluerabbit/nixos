---
name: homelab-observability
description: Investigate homelab metrics, logs, and traces via the Grafana stack (Loki / Prometheus / Tempo) — "why is X slow", "find errors in service Y", query logs/metrics, inspect dashboards, error patterns, or latency. Use whenever the user asks to query Loki/Prometheus, search logs across services, find slow requests or error spikes, check a Grafana dashboard, or correlate a runtime issue with metrics. Complements nixos-container-debug (which handles per-container journals/status). Also load homelab-topology to resolve targets and nixos-vector-logging when the issue is missing or misconfigured shipping.
---

# Homelab Observability (Grafana / Loki / Prometheus / Tempo)

Every container ships logs/metrics/traces via a Vector sidecar (wired by `_base.nix` from `cfg.logging`) to Loki + Prometheus + Tempo. Query the aggregate via the `grafana` MCP server.

## When to use this vs `nixos-container-debug`
- **This skill**: cross-service questions, time-range analysis, metrics, traces, error-pattern detection, dashboards.
- **container-debug**: live state of one container — is it up, raw journal, exec inside it.

Often: start broad here (which service is erroring / when), then drill into one container with `container_journal` / `container_exec`.

## Orientation
```
list_datasources                      # find Loki / Prometheus / Tempo datasource UIDs
search_dashboards <query>             # existing dashboards
list_loki_label_names                 # what log labels exist (service, container, host…)
list_loki_label_values <label>
list_prometheus_metric_names
```
Map a service name to its log labels via the Vector `hostType`/labels (`"container"`, `"host"`, `"network"`).

## Logs (Loki / LogQL)
```
query_loki_logs        '{service="radarr"} |= "error"'   (with datasource UID + time range)
query_loki_stats / query_loki_patterns
find_error_pattern_logs        # auto error-pattern detection for a service/time window
```
LogQL essentials: `{label="x"}` stream selector, `|= "str"` / `!= "str"` line filter, `| json` / `| logfmt` parsers, `| pattern`, rate via `count_over_time({...}[5m])`.

## Metrics (Prometheus / PromQL)
```
query_prometheus               instant or range PromQL
query_prometheus_histogram     latency/duration histograms (p50/p95/p99)
list_prometheus_label_values <metric> <label>
find_slow_requests             # surface slow requests for a service
```

## Investigation pattern — "service X is slow / erroring"
1. `list_datasources` → grab Loki + Prometheus UIDs.
2. `find_error_pattern_logs` (or `query_loki_logs '{service="X"} |~ "(?i)error|panic|fatal"'`) over the suspect window.
3. `query_prometheus_histogram` / `find_slow_requests` for latency on X.
4. Correlate timestamps; if it's one container, drill: `container_journal X unit=<svc> lines=300`, `container_status X`.
5. Check resource/network: `get_container X` for VLAN/bridge; UniFi (`list_unifi_firewall_policies`) if cross-VLAN reachability is suspect.

## Dashboards (read; create/update only if asked)
```
search_dashboards / get_dashboard_by_uid / get_dashboard_panel_queries
get_panel_image            # render a panel PNG for visual confirmation
generate_deeplink          # shareable Explore/dashboard URL
```

## Guards
- This is read/analysis. Creating dashboards, annotations, alert rules, or incidents mutates Grafana — only when explicitly asked, and state it before doing so.
- Grafana itself is a container; if the MCP can't reach it, resolve + check it with `get_container grafana` → `container_status grafana` (see nixos-container-debug).
- Always pass an explicit time range; default windows can miss the event.
