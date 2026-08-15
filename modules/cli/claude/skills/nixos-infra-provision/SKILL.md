---
name: nixos-infra-provision
description: Prepare physical/logical network infrastructure changes in this homelab via Terranix/Terraform — UniFi clients, networks/VLANs, port forwards, firewall, WiFi, and Cloudflare DNS/zones. Use whenever the user asks to add/change UniFi or Cloudflare config, a DNS record, a port forward, or a VLAN. The assistant authors aspects, previews with plan, and validates; it NEVER applies — apply is handed to the user. Also load homelab-networking when changing VLANs/bridges and den-aspects for the dual nixos/terranix aspect structure.
---

# Prepare Infrastructure Changes (Terranix / Terraform)

`modules/infra/` is the single source of truth and emits **both** NixOS configs (bridges/VLANs/firewall) and Terranix → `.tf.json` (UniFi + Cloudflare).

## ⛔ Hard gate — apply is human-only

The assistant **NEVER** runs commands that touch live infra/state:

- `just infra-apply-homelab`, `terraform apply`, `terraform import`

The assistant **may** run zero-effect commands: `just infra-plan-homelab` (read-only preview), `just eval-server`, `nix fmt`. Always end by handing the user the exact `apply` command to run themselves.

## Subdir model

Every subdir has a `_mk-*.nix` constructor beside it. It takes plain data and returns engine bodies (`mkOs` / `mkTerranix`); the aspect file keeps its identity, its `includes` and its `config` reads. **Never pass an aspect, an `includes` list, or a `config` lambda into a helper** — a helper that references `den.aspects` or `config` is wrong. Dependencies are strict: an aspect includes everything it reads (`container-dns` for networks and vpn, the VLAN for clients/ports/wlans, the port profile for devices, the zone for DNS records).

- **Dual** (`infra.*` registry for NixOS + a Terraform resource): `clients/`, `networks/`, `devices/`, `ports/`, `wlans/`, `wans/`, `vpns/`
- **Terranix-only**: `dyndns/`, `usergroups/`, `settings/`, `cloudflare/{dns,zones}/`, `gitea/repos/`
- **No resource**: `providers/` — provider blocks, version pins, sops secrets. No `resourceName`.
- `homelab.nix` = Terranix entrypoint — every resource aspect must be in its `includes` to be provisioned.

## The import-block convention (critical)

**Every applied resource carries one** — import blocks are idempotent once the resource is in state, so they stay in the config permanently. An applied resource without one is a bug.

The one exception is a resource that has never been applied: it has no controller object to adopt. `_mk-port-profile.nix` takes `unifiId ? null` and emits no import when null. Fill the id in from the apply output afterwards — until you do, a rebuilt state creates a duplicate instead of adopting.

Import id by resource type:

| resource                       | id                               |
| :----------------------------- | :------------------------------- |
| `unifi_client`, `unifi_device` | MAC                              |
| `unifi_network`                | display name (`MEDIA`)           |
| `unifi_setting`                | site name                        |
| all other `unifi_*`            | controller object id (`unifiId`) |
| `cloudflare_zone`              | `${var.cf_zone_id}`              |
| `cloudflare_dns_record`        | `<zone id>/<record id>`          |

Object ids come from `list_unifi_*` or a plan's `Refreshing state... [id=…]` lines. A resource that doesn't exist yet has no id — write it, hand off the apply, then add the import.

## Procedure

1. Write/edit the resource aspect via its `_mk-*.nix` constructor.
2. Add the aspect to `modules/infra/homelab.nix` `includes`.
3. `git add` new files; `nix fmt`.
4. `just eval-server` — catch eval/type errors in generated config.
5. `just infra-plan-homelab` — preview the UniFi + Cloudflare diff. Read it: confirm it creates exactly what's intended, with no unexpected destroys.
6. **Hand off the apply** — give the user the command; do not run it:
   ```bash
   just infra-apply-homelab    # user runs — assistant must NOT
   ```
7. After the user reports a successful apply, add the new resource's import block with the id from the plan output, then `nix fmt`.
8. `just topology` — the README diagrams are generated from these registries, and CI fails if they are stale.

## Live state inspection (read-only)

UniFi MCP tools to verify reality vs intent:

```
list_unifi_clients [type=wired|wireless]    list_unifi_networks
get_unifi_client <mac-or-name>              list_unifi_port_forwards
list_unifi_devices                         list_unifi_wlans
get_unifi_device <name>                     list_unifi_firewall_policies
                                           list_unifi_port_profiles
```

Cross-check declared infra with `get_client`, `get_vlan`, `search`.

## Common tasks

| Task                         | Where                                                                                                                                       |
| :--------------------------- | :------------------------------------------------------------------------------------------------------------------------------------------ |
| New container's UniFi client | `modules/infra/clients/container-<name>.nix` (see `nixos-container-create`); ensure it's in `homelab.nix`                                   |
| Cloudflare DNS record        | `modules/infra/cloudflare/dns/` aspect → add to `homelab.nix`                                                                               |
| Port forward                 | no `portforwards/` subdir yet — create it plus a `_mk-port-forward.nix`                                                                     |
| New VLAN                     | `modules/infra/networks/<vlan>.nix` (dual) + host bridge aspect `modules/hosts/server/networking/vlans/<vlan>.nix` + register in `host.nix` |
| WiFi SSID                    | `modules/infra/wlans/`                                                                                                                      |
| Wiring a device to a switch  | `modules/infra/ports/<vlan>-<peer>.nix` + `portProfile` on the client/device + the switch's `port_override` — see below                     |

## Port profiles are the link identity

One profile per **connection**, not per VLAN. `id` is `<vlan>-<peer>` for an access port (`trust-desktop`, `mgmt-server-ipmi`) and `<what>-trunk` for a trunk (`living-room-trunk`, `ap-trunk`). The peer half is the exact client/device aspect id, so the join reads itself.

Both ends name the profile: the switch in `extra.port_override[].port_profile_id`, and whatever is plugged in via `portProfile` on its `mkClient`/`mkDevice` call. That pairing is the only thing tying the two ends together — Terraform models no link. Two `infra.options` assertions guard it: a `portProfile` must exist in `infra.ports`, and on a non-trunk profile the client's `vlan` must equal the profile's `network`.

`resourceName` is frozen independently of `id`, so profiles can be renamed in nix with zero Terraform churn. Renaming `resourceName` moves the Terraform address and destroys/recreates the profile — use a `moved` block if you ever must.

## Guards

- Never hand-edit generated `.tf.json`. Edit aspects, regenerate.
- If `plan` shows destroys of existing clients, it usually means a missing/wrong `import` block or a changed MAC — stop and investigate before handing off the apply.
