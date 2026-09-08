# CLAUDE.md — NixOS Homelab Configuration

Modular NixOS flake for multiple hosts (`desktop`, `server`). Built on [den](https://github.com/vic/den) (aspect framework) + [denful-networks](https://github.com/icebluerabbit/denful-networks) (co-orchestrates NixOS with UniFi + Cloudflare via Terranix/Terraform). Covers system config, home-manager, custom packages, and physical network infra.

This file holds **project-wide conventions and guardrails** (always loaded). Topic detail and task procedures live in **skills**, loaded on demand — see _Skills_. Don't add procedural or subsystem detail here; put it in a skill.

---

## ⛔ Operational Safety — never touch the live system

The assistant **NEVER** runs commands that affect the running OS or live network, even temporarily:
`just switch*` / `nixos-rebuild switch`, `just test*` / `nixos-rebuild test`, `just infra-apply-homelab`, `terraform apply`/`import`.

**Allowed** (zero live effect): `just eval-*`, `just build-*` (dry-run), `just infra-plan-homelab` (read-only), `nix fmt`, `just flake`, `git add`.

Validate with eval/build/plan, then **hand off** any switch/test/apply — give the user the exact command to run themselves.

---

## Working Agreement

- **Turn-by-turn authorization** for writes to container configs: propose the exact changes, then wait for `"lgtm"`/`"sgtm"` — no other phrasing. (Detail: `nixos-container-create`.)
- **Dialogue-first**: a question or design discussion in the request → discuss; do not write files that turn, even if a command is also present.
- **Never blame undeployed changes** for an issue. Assume deployed unless there's concrete evidence otherwise.
- **Never ask for info you can fetch yourself** — status, logs, IPs, file content: use sys-mcp / SSH / the repo first.

---

## Environment

NixOS. Standard tools (`python`, `go`, `node`…) may not be on `PATH`. Bring missing tools in temporarily — never install permanently for a one-off:

```bash
nix shell nixpkgs#jq nixpkgs#curl -c bash -c 'curl ... | jq .'
```

Fetching remote repos (2+ files): shallow + blobless clone into the session scratchpad, not bare `/tmp`:

```bash
git clone --depth 1 --filter=blob:none --sparse <url> <scratchpad-path>
```

---

## Build & Workflow

- **Formatter**: `nix fmt` (treefmt — `modules/formatter.nix`) — always run before evaluating. Covers nix (nixfmt-rfc-style), python (ruff), go (gofmt), shell (shfmt + shellcheck), markdown (prettier, `*.md` only). Configs are explicit at the repo root: `ruff.toml`, `.prettierrc.toml`, `.editorconfig` (drives shfmt), `.shellcheckrc`. The pre-commit hook runs the same wrapper, so hook and `nix fmt` cannot disagree.
- **flake.nix is auto-generated** (`import-tree ./modules` auto-imports every `.nix`; `_`-prefixed paths and non-`.nix` files are ignored). Never hand-edit it. Run `just flake` after adding/removing modules.
- **Flakes ignore untracked files** — `git add` (or `git add -N`) new files before Nix can evaluate them. `just format` runs `git add -N .` first.
- Commands: `just build-*` / `eval-*` (safe checks), `just vm-*`, `just sops-host-secrets` / `sops-user-secrets`, `just gc`, `just infra-plan-homelab`, `just topology`. See _Operational Safety_ for the human-only set.
- **`just topology`** regenerates the README's Network Topology sections from `modules/infra` + the server's `networking`/`containers` (renderer: `packages/topology/`). CI fails if stale — run it after any change to a client, device, port profile or container.

---

## Topology & Diagnostics

An MCP server (**sys-mcp**, `packages/sys-mcp/`) resolves hosts / containers / clients / IPs / ports / VLANs and runs diagnostics with **auto-SSH to the correct host** — prefer it over grep or manual ssh; running a diagnostic on the wrong host returns wrong data with no error. `server` runs all nspawn containers; `desktop` runs none. Full resolution model + tool catalog: skill `homelab-topology`.

Other MCP servers: `nixos` (packages/options), `grafana`, `github`, `git`, `context7`, `fetch`, `memory`, `sequential-thinking`, `home-assistant`.

---

## Repository Layout

```
modules/
  apps/ boot/ cli/ dev/ fonts/ groups/ gui/ hw/ iso/ networking/ security/ services/ style/ users/
  cli/claude/   # this CLI's config + skills/
  containers/   # nspawn container aspects (+ _base.nix, _options.nix)
  hosts/        # per-host profiles: desktop/, server/ (host.nix, containers/, networking/, security.nix, …)
  infra/        # single source of truth for network topology → NixOS + Terranix (clients/, networks/, devices/, …)
  logging/      # Vector sidecar module
packages/       # custom packages — auto-registered, see below
secrets/        # SOPS-encrypted YAML (never commit plaintext)
```

**packages/** — every `packages/<name>/` is auto-`callPackage`d by `modules/overlays.nix` into `pkgs.homelab.<name>` and exposed as a flake output (`nix build .#<name>`). A new package is just a new directory; git-add it before Nix can see it. **`modules/` holds no `.py` or `.go`** — scripts that used to sit beside a container aspect are packages now, referenced as `lib.getExe pkgs.homelab.<name>`. Config still travels by argv/env/JSON: packages are fixed, hosts vary the flags. Referencing `pkgs.homelab.*` from inside a container body fails — see skill `nixos-container-conventions`.

**`packages/go/` is the exception to one-directory-one-package**: it is a Go workspace of several modules whose `default.nix` returns an _attribute set_, which the overlay merges into `pkgs.homelab.*` flat — so each binary is still `pkgs.homelab.<name>`. Every Go binary this repo authors lives there, one top-level directory per project, with shared code in `lib/`. Layout rules, the `go.work` registry, generated `vendor-hashes.nix` and the `just go-*` recipes: **`packages/go/README.md`**. Read it before adding or moving anything under `packages/go/`.

---

## den Aspects (the core idiom)

Everything is an **aspect**: a self-contained unit with `includes` (composition), and engine sections `nixos` / `homeManager` / `terranix` / `provides.to-hosts.nixos`.

```nix
den.aspects.<category>._.<name> = { includes = [ … ]; nixos = { … }; terranix = { … }; };
```

`_` separators are den's namespace convention, not literal name characters. Full structure, composition, and the container two-layer pattern: skill `den-aspects`.

---

## Skills

`modules/cli/claude/skills/<name>/SKILL.md`.

**Procedures (workflows):**
| Skill | Use for |
|:---|:---|
| `nixos-container-create` | Add a new container end-to-end |
| `github-repo-create` | Create + wire a new GitHub repo (binary, `-flake` wrapper, nix lib) |
| `nixos-container-debug` | Logs / status / connectivity for a running container or host service |
| `nixos-secret-add` | Add/wire a sops-nix secret |
| `nixos-infra-provision` | Prepare UniFi/Cloudflare/Terranix changes (apply handed to user) |
| `nixos-web-expose` | Caddy reverse-proxy + Homepage card for a web UI |
| `homelab-observability` | Grafana / Loki / Prometheus / Tempo investigation |

**Knowledge (reference):**
| Skill | Covers |
|:---|:---|
| `homelab-topology` | host/container/IP/VLAN resolution, sys-mcp catalog, SSH fallbacks |
| `homelab-networking` | VLAN/bridge/bond architecture, direct-bridging philosophy, MAC formula |
| `nixos-container-conventions` | `_base.nix`, port/UID/tmpfiles/firewall/glue invariants |
| `den-aspects` | den framework structure + patterns |
| `nixos-vector-logging` | Vector sidecar options + wiring |
| `nix-style` | nixfmt/statix/deadnix conformance + repo Nix idioms |

---

## Git Workflow

`main` = stable, `dev` = active. CI: `.github/workflows/ci.yml` (build check), `promote.yml` (release PR). Stage new files with `git add` before evaluating; run `just flake` after adding/removing modules.
