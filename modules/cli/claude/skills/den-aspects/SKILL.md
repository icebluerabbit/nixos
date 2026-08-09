---
name: den-aspects
description: Knowledge reference for the den aspect framework this repo is built on — aspect naming/namespacing, the includes/provides/nixos/terranix/homeManager structure, composition, and the provides.to-hosts pattern containers use to inject host options. Use when writing or editing any module/aspect, composing aspects via includes, or understanding how container options reach the host config. Also load alongside nixos-container-create or any module-writing task — pull it in whenever aspect syntax, includes, or provides.to-hosts come up, even if the main task is something else.
---

# den Aspect System (reference)

This repo uses [den](https://github.com/vic/den). Everything is an **aspect** — a self-contained unit declaring its own options, dependencies, and outputs. Modules are auto-imported by `import-tree ./modules` (every `.nix`; `_`-prefixed paths and non-`.nix` ignored).

## Naming

```
den.aspects.<category>._.<name>                    # top-level
den.aspects.hosts._.server._.containers._.radarr   # nested
```
The `_` separators are den's **namespace convention** — not literal characters in the name. Aspects usually set `aspect-name = "<category>.<name>"`.

## Structure

```nix
den.aspects.<category>._.<name> = {
  includes = [
    den.aspects.foo._.bar     # other aspects this composes with; later includes override earlier
  ];

  provides.to-hosts.nixos = { config, lib, ... }: {
    # options/config injected INTO host configurations
  };

  nixos = { config, lib, ... }: {
    # NixOS module, evaluated in the host system context
  };

  terranix = { config, ... }: {
    # Terraform/Terranix config → .tf.json for UniFi/Cloudflare
  };

  homeManager = { config, ... }: {
    # home-manager config, evaluated in the user context
  };
};
```

## Key patterns

- **`includes`** — composition. Adding an aspect merges all its `nixos` / `terranix` / `homeManager` outputs into the current evaluation. Order matters; later overrides earlier. (This is also how Caddy/Homepage host aspects pull a container's infra client into `config.infra.clients` scope.)
- **`provides.to-hosts.nixos`** — container aspects use this to inject `options.containerConfigs.<name>` into the host so the host can configure networking/storage.
- **Two-layer container pattern**: the container aspect (`modules/containers/<name>.nix`) declares options via `provides.to-hosts.nixos`; the host override (`modules/hosts/server/containers/<name>.nix`) sets them in `nixos`.

## Engines

| Section | Evaluated as | Produces |
|:--|:--|:--|
| `nixos` | NixOS module on the host | system config |
| `homeManager` | home-manager module for the user | user config |
| `terranix` | Terranix | `.tf.json` for UniFi + Cloudflare |
| `provides.to-hosts.nixos` | injected into host's NixOS eval | host-visible options |

Infra clients/networks are **dual-engine** (both `nixos` and `terranix`). Flake regeneration: `just flake` after adding/removing modules — never hand-edit `flake.nix`.
