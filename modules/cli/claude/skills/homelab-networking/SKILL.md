---
name: homelab-networking
description: Knowledge reference for this homelab's network architecture — VLAN bridges, the bonded trunk, how containers attach to VLANs, Layer-2 vs router-firewalled traffic, and the deterministic MAC address scheme. Use when adding or reasoning about a VLAN/bridge, deciding which VLAN a container belongs on, computing or validating a MAC, or understanding why two services can/can't reach each other. Also load alongside nixos-container-create when placing a container on a VLAN or assigning its MAC; pairs with homelab-topology (which looks up where things currently are).
---

# Homelab Networking (reference)

## Container bridging philosophy

- Containers bridge **directly** to a VLAN — **no host NAT**. Each is a first-class LAN client: own static IP, virtual MAC, DNS name. Traffic is routed/firewalled by the upstream UniFi router like any physical device.
- **Same bridge = Layer 2.** Two containers on the same bridge (e.g. PostgreSQL + pgAdmin on `br-services`) talk directly at L2, **bypassing the router firewall**. If you need a firewall boundary between two containers, put them on **different VLANs**.
- Container aspects are **network-agnostic**: no hardcoded IPs, ports, or VLAN knowledge inside `modules/containers/<name>.nix`. All networking is offloaded to the router/switch and wired at the host layer.

## Server interface stack

The server has a bonded trunk over its physical NICs. Each VLAN gets an 802.1Q sub-interface and a bridge:

```
bond0 (trunk)
  └── vlan-<name> (802.1Q tagged, deterministic MAC from infra client server-vlan-<name>)
        └── br-<name> (bridge, holds the host IP on that VLAN; containers attach here)
```

Each VLAN bridge is its own aspect: `modules/hosts/server/networking/vlans/<vlan>.nix`. Bridge MAC and host IP both come from infra clients (`server-vlan-<name>`, `server-bridge-<name>`), never hardcoded.

Above `bond0` is the switch: USW Pro HD ports 25+26 as one LACP group carrying the `server-trunk` port profile, which excludes only `mgmt` — the server reaches MGMT on its own NIC via port 2 instead. The two ends are joined by that profile name, declared on the switch's `port_override` and on `infra.clients.server-trunk.portProfile`. A VLAN added to the server must also stay tagged on that trunk, or the bridge comes up carrying nothing.

**Adding a container VLAN**: a bridge aspect must exist and be registered in `modules/hosts/server/host.nix` under `networking.vlans.<name>`.

## Deterministic MAC scheme

MACs derive from the **VLAN ID** so they're easy to trace. `next_free_mac <vlan>` (sys-mcp) computes a globally-unique one for you — **always use it; never hand-pick a MAC.**

> ⚠️ **The last byte is a single shared 8-bit space, NOT per-VLAN.** The container byte `vlanId + index` can collide _across_ VLANs whenever two VLAN bases are within `#containers` of each other. Real example: media (base `0x78`/120) index ≥10 lands on `0x82`+ and collides with download (base `0x82`/130) — `:82` (cross-seed vs download host) and `:84` (autobrr vs vpn-gw) both clashed. UniFi keys clients by MAC, so a duplicate makes two Terraform clients fight over one controller record (name/IP thrash). `next_free_mac` allocates from the highest index in use on the VLAN + 1 (never reusing a gap) and then skips forward over any byte already owned globally; `validate_container` fails on `mac_unique` if a duplicate slips in. If a VLAN's containers spill into the next VLAN's base, the resulting MAC is unique but no longer strictly `vlanId+index` (annotate it in the client file).

| Interface type   | OUI prefix          | Last byte (`xx`)                    | Example (VLAN 130 / DOWNLOAD)         |
| :--------------- | :------------------ | :---------------------------------- | :------------------------------------ |
| Host VLAN client | `62:f5:a4:5b:20:xx` | `vlanId` (hex)                      | `62:f5:a4:5b:20:82` (130 = `0x82`)    |
| Container client | `62:f5:a4:5b:20:xx` | `(vlanId + index)` (hex), index ≥ 1 | `62:f5:a4:5b:20:83` (130+1 = `0x83`)  |
| Host bridge      | `82:13:8a:a0:4f:xx` | `(vlanId + 32)` (hex)               | `82:13:8a:a0:4f:a2` (130+32 = `0xa2`) |

Collision-prevention rules:

1. **Container indexing**: increment `index` per container on a VLAN, starting at `1` (host VLAN client occupies index 0).
2. **Bridge offset**: bridges use the `82:13:8a:a0:4f:` OUI with a `+32` decimal offset from the VLAN ID — never overlaps VLAN/trunk interfaces.
3. **Locally-administered bit**: both OUIs have the LAA bit set (second hex digit `2`/`a`), so they never collide with real hardware MACs.

IPs are allocated only in `modules/infra/networks/<vlan>.nix` `staticAddresses` (use `next_free_ip <vlan>`). Per VLAN: `.1`-`.2` infra, `.3`-`.99` statics, `.100`-`.200` DHCP pool. `next_free_ip` takes the highest static + 1, never back-fills a gap, and ignores anything at or above the pool — a fixed-IP reservation like storage's UNAS at `.222` must not push statics past it. See `nixos-container-conventions` for the no-hardcoding rules and `nixos-container-create` for the full add procedure.
