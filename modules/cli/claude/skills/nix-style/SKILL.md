---
name: nix-style
description: Knowledge reference for writing Nix in this repo so the pre-commit linters don't rewrite or reject it afterward. Covers the gating tools — nixfmt-rfc-style (format), statix (antipattern lint), deadnix (dead-code lint) — with the concrete rewrites each one forces, plus the repo's non-enforced conventions (no `with lib;`, `lib.mkOption`/`mkIf`, file-header block comment, `lib.getExe`, no magic numbers). Also covers the non-Nix formatters `nix fmt` drives (ruff, gofmt, shfmt, shellcheck, prettier) and the 79-column rule that Python packages must satisfy to build. Load before writing or editing ANY `.nix` file, before touching a script under `packages/`, when a diff came back reformatted or a commit hook failed, or when unsure how nixfmt/statix/deadnix will treat something. Pairs with den-aspects and nixos-container-conventions.
---

# Nix Style (reference)

Three tools gate every commit for Nix specifically, via `git-hooks.nix` (see `modules/git-hooks.nix`). All are deterministic and will rewrite or reject non-conforming code — write it right the first time.

| Tool                                             | Job          | Fails commit on                                  |
| :----------------------------------------------- | :----------- | :----------------------------------------------- |
| **nixfmt-rfc-style** 1.4.0 (`nix fmt` / treefmt) | formatting   | any unformatted file                             |
| **statix** (`statix check`)                      | antipatterns | manual-inherit, useless parens, bool compares, … |
| **deadnix** (`deadnix --fail`)                   | dead code    | unused bindings / function args                  |

Always `nix fmt` before evaluating. `just format` runs `git add -N .` first (flakes ignore untracked files).

## `nix fmt` is not Nix-only

`modules/formatter.nix` drives treefmt across five languages, and the pre-commit hook runs that same wrapper — so the hook and `nix fmt` can never disagree. Each formatter reads an explicit config at the repo root.

| Language | Tool                     | Config                                    |
| :------- | :----------------------- | :---------------------------------------- |
| nix      | nixfmt-rfc-style         | none (non-configurable by design)         |
| python   | ruff-format + ruff-check | `ruff.toml`                               |
| go       | gofmt                    | none (non-configurable by design)         |
| shell    | shfmt + shellcheck       | `.editorconfig` (indent), `.shellcheckrc` |
| markdown | prettier (`*.md` only)   | `.prettierrc.toml`                        |

Excluded globally: `flake.nix` (generated), `.pre-commit-config.yaml` and `result` (store symlinks), `secrets/**` (reformatting invalidates the sops MAC), `homelab/.terraform/**`.

Prettier is deliberately restricted to `*.md`. Unrestricted it would also rewrite every JSON/YAML in the tree — including sops files and the generated Grafana dashboards.

## Python in `packages/` — 79 columns is a build gate

Python packages are built with `pkgs.writers.writePython3Bin`, which runs **flake8 with no ignores** as part of the build. flake8's default `max-line-length` is 79, so an 80-column line does not merely lint badly — **the package fails to build**. `ruff.toml` pins `line-length = 79` so the formatter targets the same width the builder enforces.

Two consequences when adding or editing a script:

- **No shebang.** `writePython3Bin` emits its own; a leftover `#!/usr/bin/env python3` lands on line 2 and fails `E265`.
- **Comments and docstrings are yours to wrap.** ruff-format splits code but never reflows a comment, docstring or string literal, so an over-long one survives the formatter and then fails the build. Wrap prose by hand; split long f-strings into adjacent literals.

Shell fragments that are `readFile`'d into `writeShellApplication` have no shebang either, so they carry `# shellcheck shell=bash` on line 1 instead.

## nixfmt — write the shape it emits

Non-configurable. Don't fight it; match it.

- **2-space indent**, no tabs. Never manually align `=` or trailing comments — nixfmt collapses to one space (`bb = 22;      # x` → `bb = 22; # x`). Hand-alignment is pure churn.
- **Empty containers get inner spaces**: `{ }` and `[ ]`, never `{}` / `[]`.
- **Attrsets with 2+ bindings always expand** one-per-line. Only a single binding stays inline (`{ x = 1; }`).
- **Lists/args expand when they don't fit** one line — one element per line. Don't hand-wrap a list that fits; don't cram one that doesn't.
- **Trailing semicolons** on every binding.
- **Don't manually line-wrap** long strings/expressions — let nixfmt decide.

Diff came back reformatted? You wrote a shape nixfmt disagrees with — adopt its output, don't re-fight it.

## statix — antipatterns it will flag

Write these the fixed way up front:

- **Manual inherit** — `{ a = a; }` → `{ inherit a; }`; `{ port = cfg.port; }` → `{ inherit (cfg) port; }`. (Repo uses `inherit (cfg) …` heavily.)
- **Useless parens** — drop parens around a bare expr: `(lib.mkIf x y)` where not needed, `default = ("foo");`.
- **Bool comparison** — `x == true` → `x`; `x == false` → `!x`.
- **Collapsible / empty let** — no `let … in let … in`, no `let in x`.
- **Useless `has`-attr** — `if x ? a then x.a else y` → `x.a or y`.
- **Legacy let** — never `let { body = …; }`.
- **Empty pattern / eta** — `x: f x` → `f`; unused whole-pattern binds.

Run `statix check` (or `statix fix` to auto-apply) if unsure — but the point is to not need it.

## deadnix — no dead code

- **No unused `let` bindings** — if you bind it, use it, or delete it.
- **No unused function args** — this is why module signatures are `_:` when the arg is unused, and only destructure what you reference: `{ den, ... }:`, `{ pkgs, lib, ... }:`. Don't pull in `config` "just in case" — deadnix fails the commit.
- **No unused `inherit`**.

## Repo idioms (NOT tool-enforced — you must)

- **No `with lib;`** anywhere. Always qualify: `lib.mkOption`, `lib.mkIf`, `lib.mkForce`, `lib.types.str`, `lib.mkEnableOption`. (0 uses in the tree — keep it.)
- **File header** — block comment naming the thing + upstream URL:
  ```nix
  /*
    mako — lightweight Wayland notification daemon
    https://github.com/emersion/mako
  */
  ```
  Aspect/host files may use a single-line `# name — one-liner` header (see `host.nix`, `formatter.nix`). Match the neighbours.
- **Options**: `lib.mkOption { type = …; default = …; description = "…"; }`; every option gets a `description`. Boolean toggles → `lib.mkEnableOption "…"`.
- **`lib.getExe pkgs.foo`** over `"${pkgs.foo}/bin/foo"` for the main binary (50+ uses).
- **Ints into strings**: `"${toString cfg.uid}"`, `"…:${toString cfg.dbPort}/…"`.
- **Container body**: single-line `let cfg = config.containerConfigs.<name>; in { … }` (see `nixos-container-create`).
- **No magic numbers/IPs/ports** — ports from `infra.clients`, IPs from `infra.vlans`, UIDs are unique options. See `nixos-container-conventions`.

## Self-check before eval

1. `nix fmt` run? 2. Any `x = x;` / `x = cfg.x;` → `inherit`. 3. Any `with lib;` → qualify. 4. Any unused arg/binding → drop it. 5. Hand-aligned `=`/comments or hand-wrapped lists → let nixfmt. 6. New file `git add -N`'d?
