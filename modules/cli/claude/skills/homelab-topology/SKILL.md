---
name: homelab-topology
description: Knowledge reference for resolving this homelab's topology — which host runs a container/service, a host or container's IP, what owns an IP or port, VLAN membership, and how to reach things via SSH. Use whenever you need to map a name/IP/port to a host/container/VLAN, decide which machine to run a command on, or pick the right sys-mcp tool. Read before running any systemctl/journalctl/nixos-container/ss/curl so it targets the correct host. Also load alongside nixos-container-debug and homelab-observability when a task needs to find or reach a host/container; pairs with homelab-networking for VLAN/MAC design.
---

# Homelab Topology Resolution (reference)

This repo manages **multiple hosts**. A diagnostic run on the wrong host returns wrong data with **no error**. Always resolve host → IP → "am I there?" before acting.

## Hosts

| Hostname  | Runs containers?            | Reach from elsewhere                                                     |
| :-------- | :-------------------------- | :----------------------------------------------------------------------- |
| `server`  | Yes — all nspawn containers | `ssh zms@10.10.20.2` (trust VLAN) or `ssh zms@10.10.10.2` (mgmt VLAN)    |
| `desktop` | No containers               | resolve its IP from `modules/infra/networks/trust.nix` `staticAddresses` |

Host IPs are **never hardcoded** — resolve from `modules/infra/networks/<vlan>.nix` `staticAddresses` (or sys-mcp).

## The rule — before any `systemctl`/`journalctl`/`nixos-container`/`ss`/`curl`

1. **Which host runs it?** Container → `modules/hosts/server/containers/<name>.nix` present = on `server`. Desktop service → `modules/hosts/desktop/`.
2. **That host's reachable IP?** → `modules/infra/networks/<vlan>.nix`.
3. **Am I on it?** → `hostname`. If not — **SSH first**, then run.

sys-mcp runtime tools (`container_*`, `host_*`) do all three automatically — prefer them.

## sys-mcp — tool → task map (use before grep)

MCP server at `packages/sys-mcp/`. Returns a fully connected entity: VLAN + client (IP/MAC/ports) + container (bridge/bind mounts) + host (SSH target) + pre-built diagnostic commands.

| Task                                             | Tool                                                                                                                 |
| :----------------------------------------------- | :------------------------------------------------------------------------------------------------------------------- |
| Nix file → topology + content                    | `resolve_path <path>`                                                                                                |
| Fuzzy search anything (name/IP/MAC/port)         | `search <query>`                                                                                                     |
| What owns this IP?                               | `get_by_ip <ip>`                                                                                                     |
| Who uses this port?                              | `get_by_port <port>`                                                                                                 |
| Full container detail + commands                 | `get_container <name>`                                                                                               |
| Full client detail + VLAN                        | `get_client <key>`                                                                                                   |
| Full VLAN detail + client list                   | `get_vlan <name>`                                                                                                    |
| Physical device                                  | `get_device <name>`                                                                                                  |
| All declared containers                          | `list_containers`                                                                                                    |
| All hosts + counts                               | `list_hosts`                                                                                                         |
| Topology overview                                | `describe_topology`                                                                                                  |
| Next free IP / MAC                               | `next_free_ip <vlan>` / `next_free_mac <vlan>`                                                                       |
| Container checklist                              | `validate_container <name>`                                                                                          |
| Container logs / status / exec / list (auto-SSH) | `container_journal` / `container_status` / `container_exec` / `container_list`                                       |
| Host journal / status / exec (auto-SSH)          | `host_journal` / `host_status` / `host_exec`                                                                         |
| UniFi live state                                 | `list_unifi_devices/clients/networks/port_forwards/wlans/firewall_policies/port_profiles`, `get_unifi_device/client` |

### What sys-mcp does NOT cover (use SSH manually)

- Creating/modifying Nix files; SOPS/secrets management
- Interactive root shell (`nixos-container root-login`)
- Arbitrary `curl` from the server
- UniFi/Cloudflare provisioning (Terraform write ops)

```bash
ssh zms@10.10.20.2 "sudo nixos-container root-login <name>"      # root shell inside a container
ssh zms@10.10.20.2 "curl -s http://<container-ip>:<port>/health" # curl from the server
ssh zms@10.10.20.2 "journalctl -u container@<name>.service -n 100" # host-level container logs
```

Containers are **bridged, not NAT'd** — their IPs are routable from anywhere on the LAN, so `curl http://<container-ip>:<port>/...` works from any host.

## Manual fallback (sys-mcp unavailable)

```bash
grep -r "<ip>" modules/infra/networks/         # IP → VLAN + name key
grep -rl "<ip>" modules/infra/clients/         # → infra client (ports)
grep -rl "container-<name>" modules/hosts/      # → which host runs it
hostname                                        # am I there? else ssh
```

## Gotcha: aspect name ≠ infra client name

Not always 1:1 — e.g. aspect `dnsmasq` ↔ client `container-dns`. sys-mcp resolves via IP linkage (`get_container dnsmasq` returns the right client). In Nix, never assume they match:

```bash
grep -rl "container-dns" modules/hosts/   # → modules/hosts/server/containers/dnsmasq.nix
```

Diagnostic procedures build on this: `nixos-container-debug`, `homelab-observability`.
