# go

Every Go binary and library this repo authors. Unlike the rest of `packages/`,
this directory is **not one package**: it is one Nix `src` containing several Go
modules, and its `default.nix` returns an attribute set of derivations rather
than a single one. `modules/overlays.nix` merges that set into the `homelab`
namespace flat, so a binary here is `pkgs.homelab.<name>` exactly as it would be
from its own directory — moving a tool in changes no callsite.

## Layout

```
lib/                  module homelab                  shared code; ships no binaries
  app/                bootstrap: flags, config, obs, signals, metrics, exit codes
  buildinfo/          Version, stamped by one ldflag
  clients/<service>/  one package per external API
  config/ httpx/ obs/ secret/
<project>/            module homelab/<project>
  main.go             package main: flags in, config struct out, hand off
  internal/<pkg>/     everything it does
```

One directory per project, one project per module, flat. There is no grouping
layer and no shared-vendor bucket.

**No `cmd/` layer.** `cmd/` disambiguates when a module ships several binaries
or a library alongside one; every project here ships exactly one binary, so it
would never disambiguate anything. The name is already supplied: a `main`
package at the module root installs as the last element of the module path, so
`module homelab/cleanuparr-setup` builds `cleanuparr-setup`. Add `cmd/` to a
project the day it grows a second binary, and nothing else changes.

**`internal/` needs a named package inside it.** `package internal` is legal but
reads as `internal.Run` at every call site. The subdirectory is also what keeps
the logic out of `package main`, which matters because coverage floors every
non-`main` package — logic in `main` would be silently exempt.

## Where does a new binary go?

Its own top-level directory. That is the whole rule — there is no category to
pick and no dependency budget to check.

This was arrived at by discarding two alternatives, both recorded here so they
are not reinvented:

- **Grouping by lifetime** (`tools/` for one-shots, `services/` for daemons)
  breaks immediately on real cases. `vpn-broker` is a long-running daemon with
  no dependencies; `flac-util` is an interactive one-shot with the heaviest
  dependency tree here. Lifetime predicts nothing about how a binary should be
  built, and `lib/app` serves one-shots and daemons through the same `Run`
  signature with no branch.
- **Grouping by domain** (`media/`, `torrent/`, `infra/`) reintroduces a
  classification decision per binary, and about a third of the inventory pulls
  two ways — `soularr-reclaim` is slskd and Lidarr, `cleanuparr-setup` is
  download cleanup and \*arr, `vpn-broker` is networking and torrenting. If it
  is ever wanted, put the group in the _directory_ only and leave the module
  path flat, so reclassifying costs one `go.work` line instead of an import
  rewrite across every consumer.

Cohesion decides placement. Vendor isolation is a consequence of it, not a rule
of its own: because each project vendors only what it imports, a dependency bump
for one binary cannot rebuild any other. The cost is duplicated vendor trees,
which is worth paying and is why `just go-hashes` exists.

## Rules

**`main.go` holds no logic.** Flags in, config struct out, hand to `internal/`.
`lib/app` exists so what remains is a struct literal and a call. Flags are
declared in `main.go`, never in `internal/` — the binary owns its command line.

**`internal/` is per project, not per module.** Go scopes `internal/` to its
parent directory, so a project's internals are unreachable from anywhere else.
Sharing goes _up_ into `lib/`, never sideways: importing another project would
need a `require` plus a `replace`, which is a visible, deliberate act in a diff.

**Secrets arrive by path.** A sops secret, a systemd `LoadCredential` file and a
plain file are the same thing to a binary: somewhere to read. The host knows
which it chose and says so on argv. Every credential flag comes as a
`--x` / `--x-file` pair so the host decides whether a value is secret — several
of this homelab's _usernames_ are. See `lib/secret`.

**stdout is the product, stderr is the log.** `topology` emits markdown, the
qBittorrent exporter emits JSON lines for Vector's `exec` source, most tools
emit nothing. Progress and diagnostics are log records, never `fmt.Println` —
`forbidigo` enforces this.

## The moving parts

**`go.work` is the registry.** `just _go-modules` reads it with
`go work edit -json`; a module listed there is linted, tested and covered, and
one that is not is invisible to the entire gate. Adding a module is one `use`
line.

**`vendor-hashes.nix` is generated.** Never edit it by hand. `just go-hashes`
sets each entry to a known-wrong value, builds, and reads the correct hash out
of the resulting mismatch. `just go-update` bumps dependencies and then runs it,
so the tree is buildable again in one command.

**`GOWORK=off` in every build.** nixpkgs' `buildGoModule` vendors each module
separately with `-mod=vendor`, and a workspace file would override that.
`go.work` is development-only — for gopls, `go test ./...` and cross-module
refactors.

**Coverage floors every package except `main`.** A project's `main.go` holds no
logic by construction, so measuring it would measure nothing; everything it
delegates to is gated. `just go-cover` derives the package list with
`go list -json`, so this needs no maintenance as projects are added.

## Working here

```
just go            lint, test (with -race) and cover every module
just go-lint       golangci-lint per module, config at .golangci.yml
just go-test       go test -race per module
just go-cover      coverage floor, default 80%
just go-update     bump dependencies, then regenerate vendor hashes
just go-hashes     regenerate vendor hashes alone
```

`-race` runs here rather than in the derivation because the race detector needs
cgo, which `buildGoModule` disables. `golangci-lint` is not part of `nix fmt`
either: it needs the module graph, not a file list. `gofmt` _is_ in `nix fmt`,
through treefmt, along with every other language in the repo.

New files must be `git add`ed before Nix can see them — flakes ignore untracked
files.

## Adding a project

1. `mkdir <name>` and write `main.go` against `lib/app`, with the work in
   `<name>/internal/<pkg>/`.
2. `go.mod` declaring `module homelab/<name>`, with `replace homelab => ../lib`.
3. Add `./<name>` to `go.work`.
4. Add an entry to `default.nix`, and a placeholder line `<name> = "";` to
   `vendor-hashes.nix` — `just go-hashes` rewrites an existing line, it does
   not create one.
5. `git add`, then `just go-hashes`, then `just go`.

The binary name comes from the module path, which is the project directory,
which is also the `pkgs.homelab.<name>` attribute — so a migrated tool keeps the
name its callsites already use.
