---
name: nixos-vector-logging
description: Knowledge reference for the Vector logging/metrics/traces sidecar used by every container and host in this homelab — the cfg.logging options and how to wire Loki/Prometheus/Tempo addresses from infra clients. Use when enabling or adjusting log/metric/trace shipping for a container or host, adding a custom Vector source/scrape, or troubleshooting why a service isn't appearing in Loki/Prometheus. Also load alongside nixos-container-create (new containers usually enable logging) and homelab-observability when wiring or debugging shipping.
---

# Vector Logging Sidecar (reference)

Every container and host can ship logs/metrics/traces via a [Vector](https://vector.dev) agent. `_base.nix` wires it automatically from `cfg.logging` (see `nixos-container-conventions`).

## Options (`modules/logging/_options.nix`)

| Option              | Type     | Purpose                                                            |
| :------------------ | :------- | :----------------------------------------------------------------- |
| `enable`            | bool     | Enable the Vector sidecar                                          |
| `lokiAddress`       | str      | Loki push URL                                                      |
| `prometheusAddress` | str      | Prometheus remote-write URL                                        |
| `tempoAddress`      | str      | Tempo OTLP HTTP URL                                                |
| `lokiInputs`        | list str | Vector source IDs pushed to Loki (default `["journal"]`)           |
| `prometheusInputs`  | list str | Vector source IDs pushed to Prometheus                             |
| `hostType`          | str      | Log-classification label (`"container"`, `"host"`, `"network"`)    |
| `extraSettings`     | attrs    | Raw Vector config (sources/transforms/sinks) merged in             |
| `environmentFile`   | path     | Env file for the Vector unit (runtime secrets)                     |
| `validateConfig`    | bool     | Build-time config validation (disable when using runtime env vars) |

## Wiring (host-side container config)

Addresses always come from the observability containers' infra clients — never hardcoded:

```nix
containerConfigs.radarr.logging = {
  enable = true;
  lokiAddress       = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
  prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
  tempoAddress      = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
};
```

## Custom source / scrape (extraSettings)

Example — scrape a service's own Prometheus metrics and push them (from Caddy's host config):

```nix
logging = {
  prometheusInputs = [ "caddy_metrics" ];
  extraSettings.sources.caddy_metrics = {
    type = "prometheus_scrape";
    endpoints = [ "http://localhost:${toString config.infra.clients.container-caddy.ports.admin}/metrics" ];
  };
};
```

## Troubleshooting

- Not in Loki/Prometheus → check `enable`, that the address resolves (infra client IP/port), and that the source ID is in `lokiInputs`/`prometheusInputs`.
- Build fails validating config that uses runtime env vars → set `validateConfig = false` and supply `environmentFile`.
- Query the resulting data with `homelab-observability`.
