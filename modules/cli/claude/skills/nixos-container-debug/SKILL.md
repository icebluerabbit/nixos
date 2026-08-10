---
name: nixos-container-debug
description: Diagnose a running container or host service in this NixOS homelab — check logs, service status, connectivity, or "why is X down / erroring / not reachable". Use whenever the user asks to check logs, see if a service/container is running, debug a failing unit, inspect a container's filesystem/processes, or investigate runtime issues. Resolves which host runs the thing and SSHes automatically. Also load homelab-topology (host/IP resolution) and homelab-observability (metrics/traces/aggregate logs) when relevant.
---

# Debug a Container / Host Service

Resolve **which host runs it** before any diagnostic — wrong host returns wrong data with no error. The resolution model, host map, sys-mcp tool catalog, and manual SSH fallbacks are in **`homelab-topology`**. sys-mcp runtime tools do the resolution + SSH automatically; prefer them.

## Procedure

### 1. Identify

```
get_container <name>     # IP, MAC, ports, VLAN, host, bridge, bind mounts, pre-built commands
search <query>           # if name/IP/port unknown;  get_by_ip / get_by_port for reverse lookup
```

Aspect name may differ from client name — `get_container <aspect-name>` resolves it (→ `homelab-topology`).

### 2. Is it up?

```
container_status <name>          # host unit container@<name>.service (auto-SSH)
container_list [host]            # running/stopped/failed for all containers
```

### 3. Logs (auto-SSH to correct host)

```
container_journal <name>                                # last 100, all units
container_journal <name> unit=<svc>.service lines=200
host_journal <host> unit=<unit> lines=200              # host-level unit
host_status <host> <unit>
```

### 4. Inside the container

```
container_exec <name> "systemctl --failed"
container_exec <name> "ss -tlnp"
container_exec <name> "cat /etc/resolv.conf"
```

### 5. Connectivity (containers are bridged → IPs routable from any LAN host)

```bash
curl http://<container-ip>:<port>/health
```

For interactive root shell or curl-from-server, use the manual SSH patterns in `homelab-topology`.

## Common diagnoses

| Symptom                         | Check                                                                                                             |
| :------------------------------ | :---------------------------------------------------------------------------------------------------------------- |
| Unit `activating`/`failed`      | `container_journal <name> unit=<svc>.service` for the trace                                                       |
| Service up, port unreachable    | firewall — `_base.nix` opens only `cfg.port`; multi-port needs a manual rule (→ `nixos-container-conventions`)    |
| DNS fails inside container      | `container_exec <name> "cat /etc/resolv.conf"`; nameservers come from `cfg.nameservers`                           |
| Permission denied on bind mount | host UID/GID vs container service user; the `Z` tmpfiles rule restores ownership on boot                          |
| Can't reach another container   | same bridge = L2; different VLAN = router firewall (→ `homelab-networking`); check `list_unifi_firewall_policies` |

## Do NOT

- Assume the change wasn't deployed / blame a forgotten `switch`. Assume deployed unless the journal shows concrete evidence.
- Ask the user for status/logs/IPs you can fetch yourself — tool/SSH first.

For metrics, traces, and aggregate log queries (Loki/Prometheus/Tempo) use `homelab-observability`.
