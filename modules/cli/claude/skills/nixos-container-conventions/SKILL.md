---
name: nixos-container-conventions
description: Knowledge reference for the mandatory patterns every systemd-nspawn container in this repo must follow — what _base.nix/_options.nix provide, named-port declaration and self-vs-cross reference rules, unique UID/GID, tmpfiles d+Z, firewall decoupling, host-mediated inter-container glue, ephemeral roots, passing secrets via systemd credentials, and OpenTelemetry. Use when writing or reviewing any container aspect or host override, or when unsure how to reference a port/IP/UID correctly. Load it whenever creating or editing a container — including alongside nixos-container-create — not only when conventions are explicitly asked about.
---

# Container Conventions (reference)

The invariants that keep containers decoupled, collision-free, and declarative. `nixos-container-create` is the procedure; this is the rulebook it cites.

## `_base.nix` — imported by every container

Provides automatically:
- `networking.enableIPv6 = false`
- `networking.useHostResolvConf = false` (prevents host DNS leakage)
- Static nameservers from `cfg.nameservers` (fallback: gateway + `1.1.1.1`)
- Default gateway from `cfg.gateway`
- Firewall: opens `cfg.port` TCP (pass `hasPort = false` to skip)
- Vector logging sidecar wired from `cfg.logging` (see `nixos-vector-logging`)
- Optional lldpd on `eth0` when `cfg.lldpd.enable = true`
- `system.stateVersion = "26.05"`

```nix
imports = [ (import ./_base.nix { inherit lib cfg; }) ];
```
**Multi-port / no-port** containers pass `hasPort = false` and open ports manually:
```nix
imports = [ (import ./_base.nix { inherit lib cfg; hasPort = false; }) ];
networking.firewall.allowedTCPPorts = [ cfg.httpPort cfg.httpsPort ];
```
`_options.nix` provides the shared host-side option set (`hostBridge`, `macAddress`, `localAddress`, `gateway`, `bindMounts`, `uid`, `gid`, `port`, `nameservers`, `logging`, `lldpd`, …) via `import ./_options.nix { inherit lib; defaultPort=…; defaultUid=…; defaultGid=…; }`.

## Named ports — single source of truth

Every exposed port is declared in `modules/infra/clients/container-<name>.nix` under `ports = { … }` (typed `attrsOf port`). `default` for single-port; descriptive names (`http`, `https`, `grpc`, `otlp-http`, `admin`, `web`, `ui`…) for multi-port. **Port integers appear as literals only here** — never hardcode them anywhere else, never silently rely on a module default.

Host config must assign every port explicitly from the client:
```nix
containerConfigs.radarr.port = config.infra.clients.container-radarr.ports.default;
containerConfigs.caddy = {
  httpPort  = config.infra.clients.container-caddy.ports.http;
  httpsPort = config.infra.clients.container-caddy.ports.https;
  adminPort = config.infra.clients.container-caddy.ports.admin;
};
```

### Port reference rule — self vs cross
| Reference | Use | Example |
|:--|:--|:--|
| **Self** (container → its own port) | `config.containerConfigs.<name>.<portOption>` | `"http://localhost:${toString config.containerConfigs.loki.port}/metrics"` |
| **Cross** (one service → another's port) | `config.infra.clients.container-<other>.ports.<key>` | `dbPort = config.infra.clients.container-postgres.ports.default;` |
Cross-container references always come from `infra.clients`, symmetric with IPs.

## IP allocation
Only in `modules/infra/networks/<vlan>.nix` `staticAddresses`. Reference via `config.infra.vlans.<vlan>.staticAddresses.<name>`. Never hardcode an IP in a host or client config.

## UID/GID
Every container gets a **unique static UID and GID** (search the repo before assigning — no reuse). Never use magic numbers in host config — expose `uid`/`gid` options and reference `${toString cfg.uid}` / `${toString cfg.gid}`.

## tmpfiles — pair `d` with `Z`
```nix
systemd.tmpfiles.rules = [
  "d /var/lib/appdata/<name> 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
  "Z /var/lib/appdata/<name> 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
];
```
`Z` recursively restores ownership/permissions on every boot, fixing any drift.

## Firewall decoupling
Set `openFirewall = false` on every service inside a container — the container perimeter is explicit. `_base.nix` opens `cfg.port`; multi-port opens manually (above).

## Ephemeral root
`ephemeral = true` always — the container root FS stays read-only/declarative; state lives only in bind mounts.

## Inter-container glue — host-mediated only
- Container aspects **never reference each other** — they're unaware of each other's existence.
- All wiring (db host, SCGI, upstreams) is done at the host override (`modules/hosts/server/containers/<name>.nix`) using the infra client registry: `config.infra.clients.container-<other>.ip` / `.ports.<key>`.
- Communicate over **IP**, not shared Unix sockets / bind-mounted dirs (keeps filesystem permissions decoupled).
- To avoid duplicating values (db name/user), the host queries the consumer's options (`config.containerConfigs.radarr.dbUser`) and passes them to the provider (`containerConfigs.postgresql`).

## Secrets via systemd credentials
Don't bind-mount raw secret files when they can leak into store closures. Use `--load-credential`:
```nix
extraFlags = [ "--load-credential=pgadmin-password:${cfg.initialPasswordFile}" ];
# container reads:  /run/host/credentials/pgadmin-password
```
For env-file secrets use `cfg.envFile` (see `nixos-secret-add`).

## OpenTelemetry (`enableTelemetry`)
Services with native OTel (Caddy, Grafana) expose `enableTelemetry`. When true, the service targets the local Vector sidecar (`127.0.0.1:4317` gRPC / `4318` HTTP). Enable on the host override.

## Dual-engine infra aspects
Infra clients and networks carry **both** `nixos` and `terranix` sections — local interface setup vs UniFi/Cloudflare provisioning. Keep the two sections in sync.
