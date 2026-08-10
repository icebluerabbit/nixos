---
name: nixos-container-create
description: Create or scaffold a new systemd-nspawn container in this NixOS homelab — add a service on a VLAN, register its infra client (IP/MAC/ports), write the container aspect and host override, and wire it into the host and Terranix. Use whenever the user asks to "add a container", "create a service", "run X in a container", or stand up a new bridged service. The end-to-end checklist; also load nixos-container-conventions, homelab-networking, homelab-topology, nixos-vector-logging, and den-aspects for the underlying rules and patterns.
---

# Create a NixOS Container

End-to-end procedure. The rules behind each step live in knowledge skills — load them as needed:

- **`nixos-container-conventions`** — `_base.nix`/`_options.nix`, port/UID/tmpfiles/firewall/glue invariants.
- **`homelab-networking`** — VLAN/bridge architecture + MAC formula.
- **`homelab-topology`** — resolving hosts/IPs/clients.

## ⛔ Hard gates — read before any file write

Override everything (also in CLAUDE.md):

1. **VLAN selection** — if the target VLAN isn't explicitly stated, STOP and ask. Suggest, but approval is not implied.
2. **Turn-by-turn authorization** — before any file write, output the exact proposed changes + justification and ask _"Do you authorize these changes for this turn?"_. Wait for `"lgtm"`/`"sgtm"` — no other phrasing.
3. **Dialogue gating** — any question/design discussion in the request → dialogue only, no writes that turn.

## Step 0 — Allocations (tools, not guesses)

```
next_free_ip <vlan>     → lowest unused host IP ≥ .3
next_free_mac <vlan>    → deterministic container MAC + expected bridge MAC
```

Pick a **unique** static UID/GID — search first: `grep -rn "defaultUid\|defaultGid" modules/containers/`. (MAC scheme → `homelab-networking`; UID/port/no-magic rules → `nixos-container-conventions`.)

## Step 1 — VLAN static address — `modules/infra/networks/<vlan>.nix`

```nix
staticAddresses = { gateway = "10.10.120.1"; server = "10.10.120.2"; radarr = "10.10.120.3"; };  # ← from next_free_ip
```

Only place an IP literal appears.

## Step 2 — Infra client — `modules/infra/clients/container-<name>.nix`

Both `nixos` and `terranix` sections, kept identical. Register MAC/IP/named ports (+ `proxy.port` if web-facing → `nixos-web-expose`).

```nix
{ den, ... }:
let name = "Radarr Container"; mac = "62:f5:a4:5b:20:79"; in   # mac from next_free_mac
{
  den.aspects.infra._.clients._.container-radarr = {
    includes = [ den.aspects.infra._.providers._.unifi den.aspects.infra._.options ];
    nixos = { config, ... }: {
      infra.clients.container-radarr = {
        inherit name mac;
        ip = config.infra.vlans.media.staticAddresses.radarr;
        ports = { default = 7878; };
        proxy.port = "default";        # web-facing only — auto Caddy vhost
      };
    };
    terranix = { config, ... }: {
      infra.clients.container-radarr = {
        inherit name mac;
        ip = config.infra.vlans.media.staticAddresses.radarr;
        ports = { default = 7878; };
        proxy.port = "default";
      };
      resource.unifi_client.container_radarr = {
        inherit mac name;
        fixed_ip = config.infra.vlans.media.staticAddresses.radarr;
        allow_existing = true; blocked = false; site = "default";
        skip_forget_on_destroy = false;
      };
      # import = [ { to = "unifi_client.container_radarr"; id = mac; } ];  # uncomment AFTER user's first apply
    };
  };
}
```

Named ports = single source of truth; `import` stays commented until applied (→ `nixos-infra-provision`).

## Step 3 — Container aspect — `modules/containers/<name>.nix`

Declares host options via `provides.to-hosts.nixos` (imports `_options.nix`), then the nspawn container importing `_base.nix` (what it provides → `nixos-container-conventions`).

```nix
{ den, lib, ... }:
{
  den.aspects.containers._.radarr = {
    includes = [ ];
    provides.to-hosts.nixos = { config, lib, ... }: {
      options.containerConfigs.radarr = (import ./_options.nix {
        inherit lib; defaultPort = 7878; defaultUid = 5003; defaultGid = 5003;
      });
      config = {
        containers.radarr = let cfg = config.containerConfigs.radarr; in {
          autoStart = true; ephemeral = true; privateNetwork = true;
          hostBridge = cfg.hostBridge; localMacAddress = cfg.macAddress;
          localAddress = cfg.localAddress; bindMounts = cfg.bindMounts;
          config = { pkgs, lib, ... }: {
            imports = [ (import ./_base.nix { inherit lib cfg; }) ];   # hasPort=false for multi/no-port
            users.users.radarr = { uid = lib.mkForce cfg.uid; group = "radarr"; isSystemUser = true; };
            users.groups.radarr.gid = lib.mkForce cfg.gid;
            services.radarr = { enable = true; openFirewall = false; user = "radarr"; group = "radarr"; };
          };
        };
      };
    };
  };
}
```

## Step 4 — Host override — `modules/hosts/server/containers/<name>.nix`

Network-specific values, storage, secrets, logging. All inter-container glue lives here (→ `nixos-container-conventions`; logging → `nixos-vector-logging`).

```nix
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.radarr = {
    includes = [
      den.aspects.containers._.radarr
      den.aspects.infra._.clients._.container-radarr
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.networks._.media
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];
    nixos = { config, ... }: let cfg = config.containerConfigs.radarr; in {
      containerConfigs.radarr = {
        lldpd.enable = true;
        hostBridge = "br-media";
        macAddress = config.infra.clients.container-radarr.mac;
        localAddress = "${config.infra.clients.container-radarr.ip}/${toString config.infra.vlans.media.subnetPrefix}";
        gateway = config.infra.vlans.media.staticAddresses.gateway;
        port = config.infra.clients.container-radarr.ports.default;
        bindMounts = {
          "/var/lib/radarr" = { hostPath = "/var/lib/appdata/radarr"; isReadOnly = false; };
          "/mnt/media"      = { hostPath = "/mnt/media"; isReadOnly = false; };
        };
        envFile = config.sops.templates."radarr-env".path;     # → nixos-secret-add
        nameservers = [ config.infra.clients.container-dns.ip "1.1.1.1" ];
        logging = {
          enable = true;
          lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
          prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
          tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
        };
      };
      systemd.tmpfiles.rules = [
        "d /var/lib/appdata/radarr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
        "Z /var/lib/appdata/radarr 0770 ${toString cfg.uid} ${toString cfg.gid} - -"
      ];
    };
  };
}
```

Startup deps (e.g. needs postgres): add the provider aspect to `includes` and

```nix
systemd.services."container@radarr" = { wants = [ "container@postgresql.service" ]; after = [ "container@postgresql.service" ]; };
```

## Step 5 — Register

- `modules/hosts/server/host.nix` → add `den.aspects.hosts._.server._.containers._.radarr` to `includes`.
- `modules/infra/homelab.nix` → add `den.aspects.infra._.clients._.container-radarr` to `includes`.
- New VLAN? bridge aspect must exist + be registered (→ `homelab-networking`).

## Step 6 — Web-facing? → `nixos-web-expose`

Set `proxy.port` on the infra client (step 2) → Caddy auto-proxies; DNS is automatic. Homepage card + Caddy `includes` are the `nixos-web-expose` skill.

## Step 7 — Stage, format, validate (zero-effect commands)

```bash
git add modules/infra/clients/container-radarr.nix modules/containers/radarr.nix \
        modules/hosts/server/containers/radarr.nix    # flakes ignore untracked files
just flake && nix fmt
validate_container radarr     # vlan addr, client, ports, IP/VLAN match, MAC formula, host decl, sources
just eval-server              # fast eval — fix every failing check before handing off
```

**Deploy is human-only.** The assistant never runs `switch*`/`test*`/`infra-apply`. After eval passes, hand the user `just switch-server`.

All invariants referenced above are detailed in `nixos-container-conventions` — follow them exactly.
