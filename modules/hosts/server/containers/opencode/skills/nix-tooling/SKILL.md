---
name: nix-tooling
description: How to get tools on this NixOS machine, and what survives a restart. Use when a command is "not found", when you are about to install a package, or before writing a file you expect to still be there later.
---

# Tools and persistence on this machine

This is a NixOS container. There is no `apt`, no `brew`, no installing into the
system. Project toolchains are already handled for you — see the last section.

## Getting a tool

Already on `PATH`, always:

`git` `ssh` `direnv` `rg` (ripgrep) `fd` `jq` `curl` `make` `bash` `nix`
plus coreutils, findutils, grep, sed.

Prefer `rg` over `grep -r` and `fd` over `find` — they respect `.gitignore` and
are far faster over the workspace.

For anything else, run it straight from nixpkgs without installing:

```bash
nix shell nixpkgs#<package> -c <command>
```

```bash
nix shell nixpkgs#python3 -c python3 script.py
nix shell nixpkgs#yq -c bash -c 'yq -o json f.yaml | jq .foo'
```

The `-c` is required. Without it you get an interactive shell with nothing
attached to it, and the command hangs.

`nixpkgs` is pinned to the revision this machine is built from, so it resolves
locally and offline. Write `nixpkgs#foo` — no version, no URL.

Unsure of the attribute name? `nix search nixpkgs <term>`. It usually matches the
command but not always (`rg` lives in `ripgrep`).

## Never install permanently

Do not run `nix profile install`. Do not edit `/etc` to add a package.

`/home/opencode` persists, so a profile mutation outlives every restart, drifts
from the declarative config that is supposed to fully describe this machine, and
nothing ever rebuilds or collects it. `nix shell` gets the same tool for one
command and leaves nothing behind.

If you find yourself reaching for the same `nix shell` repeatedly, say so — that
tool belongs in the container's package list, which a human edits in the `nixos`
repo.

## What survives a restart

The container is ephemeral. On restart the root filesystem is rebuilt from the
declarative config, and anything written outside these two paths is gone:

| Path                 | Survives | Notes                                                                |
| :------------------- | :------- | :------------------------------------------------------------------- |
| `/var/lib/workspace` | yes      | project checkouts; shared with the code-server IDE and the host user |
| `/home/opencode`     | yes      | your config, credentials, session state                              |
| everything else      | **no**   | `/tmp`, `/etc`, `/var`, anything installed                           |

Scratch files in `/tmp` are fine and self-cleaning. Real work belongs in
`/var/lib/workspace`, and anything you would be sorry to lose should be committed
and pushed rather than merely saved.

## Project environments are automatic

If a project has an `.envrc`, its devshell is already loaded for every command
you run inside that directory — correct compilers, language servers, and
formatters, at the versions the project pins. You do not need `direnv allow`,
`direnv exec`, or `nix develop`; that is wired into the shell itself.

Two things follow. A tool that exists inside one project but not another is
expected, not a fault. And if a command reports the wrong version of something,
check that your working directory is actually inside the project before
investigating anything else.
