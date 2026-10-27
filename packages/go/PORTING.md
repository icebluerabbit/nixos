# Porting `packages/*` into `packages/go`

Plan of record for moving every binary this repo authors into the Go workspace.
Written 2026-09-26. Implementation happens project by project in later
sessions; this file is the checklist and the decision record.

Read `README.md` in this directory first — it defines the layout, the rules and
the `just go-*` gate that every ported project has to satisfy. This file only
says _what_ moves, in what order, and what each one has to preserve.

## Decisions taken

- **Everything we author moves**, including the shell scripts. The four
  packages that only build upstream software stay where they are.
- **Multi-binary projects keep one module and one `pkgs.homelab.*` attribute**,
  with a `cmd/<name>/` directory per binary. `README.md` already sanctions
  `cmd/` for a project that ships a second binary. This keeps every existing
  `modules/` callsite and every binary name unchanged.
- **One project at a time**, each self-contained: port, tests, `default.nix`
  entry, vendor hash, deletion of the old `packages/<name>/`, green gate. The
  next project does not start until the gate passes on the current one.

## Progress

- **Wave 0 — done.** `mkProject` takes `runtimeInputs` (wraps each installed
  binary's PATH with `makeWrapper`) and `programs` (the binaries a module
  installs, for the wrapper loop and `meta.mainProgram`).
- **`topology` — done.** Output verified byte-identical to the old binary on a
  live `nix eval` dump, both views; `just topology` leaves README unchanged.
- **`mousetrap` — done.** Full lib/app adoption: `config` declares the flags
  and lib/config resolves them onto `types.Config` by json key, the collector
  registers on `env.Metrics`, and `server.Run` serves with graceful shutdown.
  `internal/config/mapping_test.go` asserts that every one of the ~60 flags
  reaches its field, and that no declared flag escapes that assertion.
  Smoke-tested with the server container's real `ExecStart` argv.

  Surface changes to carry into the handover, all additive: `/metrics` gains
  40 `go_*` / `process_*` families from `lib/obs`'s registry (no
  `mousetrap_*` or `mam_*` series renamed or dropped); the standard `lib/app`
  flags are now accepted; `HOMELAB_*` and `HOMELAB_MOUSETRAP_*` are now a
  configuration layer, though the unit sets neither; logs moved from stdout to
  stderr and carry `service` and `version`; `SIGTERM` now drains in-flight
  requests and exits 0 instead of killing the process. The MAM dashboard
  needed no query changes — its Loki panel has no `stream` selector — only the
  comment describing the stream.

- **`sys-mcp` — done.** Moved to `homelab/sys-mcp` with the
  packages under `internal/`, `main.go` on `lib/app`, and the stdio server in
  `internal/serve`. Configuration keeps its own environment names
  (`SYS_MCP_*`, `UNIFI_*`, `PROMETHEUS_URL`, `LOKI_URL`) rather than moving to
  `lib/config`'s `HOMELAB_`-prefixed layer, because those names are the
  contract `modules/cli/mcp.nix` passes.

  Verified by golden diff of the MCP tool catalog — name, input properties and
  required fields per tool — captured from the pre-port binary in both
  registration modes: 30 tools unconfigured, 39 with `UNIFI_*` set. Both
  identical after the port, so the conditional registration of the nine UniFi
  tools survives. Two read-only tools were also diffed end to end and match
  once array order is canonicalised; that ordering is nondeterministic in the
  original too.

  **Coverage: 38.6% → 84.3%**, so the temporary per-module floor was removed
  again and sys-mcp is held to the standard 80 like everything else. The tests
  drive tools through the server with a `tools/call` frame rather than calling
  handler closures, which is the only path that exercises registration, input
  schema and handler together. New fakes: an httptest UniFi controller (both
  the classic envelope and the v2 bare-array shape the firewall policies use),
  a combined Prometheus/Loki stub, and a recorder swapped in for the exec seam
  in `runtime.go` — `runcmd` is now a variable so a test can assert the command
  a diagnostic builds and which host it would have run on, without running it.

- **`flac-util` — done.** Moved to `homelab/flac-util`; the cobra interface
  went to `internal/cmd/` so it is not confused with the workspace's `cmd/`
  convention, which means binary directories. It is the one project that does
  **not** adopt `lib/app`: it is a two-subcommand CLI (`view`, `collect`) and
  `lib/app` builds exactly one command with no `AddCommand`, so adopting it
  would have meant either extending the shared library or flattening the
  interface. It also keeps its own `Version` const rather than
  `lib/buildinfo`, whose value is `"dev"` outside Nix and is written into
  every report. Wave 0's `runtimeInputs` carries the `flac` and `chromaprint`
  PATH wrap the old `default.nix` had.

  Coverage was already 91.9% and needed nothing. Lint was 32 issues and is now
  zero — mostly gosec annotations that are load-bearing to explain: MD5 here
  is the FLAC format's own StreamInfo checksum, and several integer
  conversions are deliberate two's-complement reinterpretations rather than
  value conversions. One gocritic suggestion in `report.Write` was **rejected**
  and suppressed instead: `err =` feeds the named return the deferred cleanup
  tests, so `err :=` would have leaked a temp file after a failed write.

  Parity: `--help` for the root and both subcommands byte-identical, and all
  six FLAC fixtures re-measured — 82 fields per report identical once the
  three run-dependent ones (`generated`, `file.path`, `file.modified`) are
  excluded.

- **Wave 2 — done.** `thelounge-seed`, `forgejo-token-inject`,
  `qui-provision`, `hermes-resolve-rooms`, `nixos-notify`, `tuwunel-init`,
  `soularr-reclaim` and `ha-seeders` all ported, each verified by golden
  capture against the Python original and each deployed and checked live on
  `server`.

- **`qbt-scripts` — done.** Three binaries over one module, so
  `pkgs.homelab.qbt-scripts` keeps its name and all three callsites are
  unchanged. The shared WebAPI surface went into `lib/clients/qbittorrent`
  rather than a private client: that package already talks to this service,
  and a second qBittorrent client in the same repo is how the two drift. It
  gained `Peers`, `Reannounce`, `AddTorrentFile`, `TorrentsFiltered` and one
  decoded rate field, `Torrent.UpSpeed`, which the exporter ranks on.

  `qbt-reannounce` and `qbt-watch` run on `lib/app`; their flag names and
  values are unchanged, so neither unit moved. `qbt-peer-exporter` does
  **not**, for two reasons that are both about Vector's `exec` source: it
  takes its arguments positionally, which `lib/app` rejects, and it must
  write nothing to stderr, because the source turns stderr lines into events
  that the `parse_json!` transform behind it would then fail on. It has no
  logger at all; `scrape_ok` is how a failed pass reports itself.

  The 20 JSON field names the `log_to_metric` transform reads are asserted in
  `internal/peers`, because renaming one does not fail anywhere — the gauge
  just stops being emitted.

  Parity against the three Python originals on a recording stub: identical
  WebAPI request sequences for reannounce (8 torrents, 2 selected) and watch
  (3 imports, multipart fields, filename, part content type and bytes all
  equal), identical resulting directory trees, and identical exporter output
  — same 20 keys in the same order with the same values, uncapped, capped at
  2 (same ranking, same two torrents visited) and with qBittorrent down
  (`scrape_ok: 0`, empty stderr, exit 0). Only the whitespace between JSON
  tokens differs, which Vector parses away.

  Deviation worth knowing: a failed backup move is now logged and skipped,
  where the Python crashed the pass. The torrent is already in the client at
  that point, so continuing is right.

- **`beets-lidarr-import` — done.** The Lidarr surface went to
  `lib/clients/arr`, which already talks to the family: `ManualImport`,
  `NewManualImport`, `RunCommand`, `CommandStatus` and `Command.Finished` /
  `Command.Detail`. `ManualImport` refuses a Sonarr or Radarr client rather
  than decoding their differently-shaped listing into zeroed ids, which would
  look merely unresolved. The quality object round-trips as
  `json.RawMessage`: re-encoding it through a struct would quietly import a
  different quality than the one identified.

  The one callsite change in the whole wave: the beets hook now passes
  `--album {album.path}` instead of a bare positional, because `lib/app`
  rejects positionals. beets' `hook` plugin shlex-splits the template before
  substituting, so an album path with spaces is still one argument.
  `LIDARR_URL`, `LIDARR_API_KEY`, `LIDARR_IMPORT_MODE`,
  `LIDARR_IMPORT_TIMEOUT` and `LIDARR_IMPORT_POLL` keep their names and are
  read directly — the key especially must not become a flag, since argv is
  readable through `/proc`.

  All 20 Python test cases ported, split between `lib/clients/arr` (listing
  shape, placement, command body) and `internal/importer` (the run). Parity
  on seven scenarios against a recording stub — happy path, partly-placed
  album, empty listing, unsuccessful result, failed command, rejected
  request, three-step poll — with identical request sequences, bodies and
  exit codes throughout. The only wire difference is query-parameter order,
  which Go sorts and Python leaves in insertion order.

  Two deviations: the give-up message prints Go's duration form (`5m0s`) in
  place of `300s`, and progress moved from stdout to structured stderr with
  the error text carried verbatim inside the record.

Next: wave 4 — the vpn-broker trio.

## Stays in `packages/`

Upstream software packaged here, no code of ours to port:
`bindery`, `seasonpackarr`, `soularr`, `opencode-plugin-otel`.

When the last of our own packages moves out, `packages/` holds these four plus
`go/`.

## Inventory

LOC counts are source lines including tests, measured 2026-09-26.

### Already Go — move only

| Project     |   LOC | Notes                                                                                                                                                                                                                                                                               |
| :---------- | ----: | :---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `topology`  |  1061 | `module topology` → `homelab/topology`. Invoked by `just topology` as `nix run .#topology`; `just topology-watch` already watches `packages/go`. stdout is markdown — the product.                                                                                                  |
| `mousetrap` |  6439 | `module mousetrap` → `homelab/mousetrap`. Container consumer: `modules/containers/mousetrap/mousetrap.nix`. Has its own `README.md` referencing `packages/mousetrap` — update the path.                                                                                             |
| `sys-mcp`   |  8080 | `module sys-mcp` → `homelab/sys-mcp`. Consumers: `modules/cli/mcp.nix`, doc reference in `modules/overlays.nix`.                                                                                                                                                                    |
| `flac-util` | 19049 | `module flac-util` → `homelab/flac-util`. Wraps PATH with `flac` and `chromaprint`; needs the `runtimeInputs` support added in wave 0. Cobra `cmd/` package is command wiring, not binary directories — it stays as is. `modules/formatter.nix` mentions the old path in a comment. |

### Python — real ports

| Project                   | LOC | Binaries | Consumer                                                                                                      | Notes                                                                                                                                                                                                                                                                           |
| :------------------------ | --: | :------- | :------------------------------------------------------------------------------------------------------------ | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `beets-lidarr-import`     | 582 | 1        | `modules/containers/beets/beets.nix`                                                                          | The only package with existing tests (`beets_lidarr_import_test.py`). Port the tests first and use them as the parity oracle.                                                                                                                                                   |
| `vpn-broker`              | 594 | 1        | `modules/containers/vpn-gw/vpn-gw.nix`                                                                        | Flask HTTP service, the only non-stdlib Python dependency in the whole set. Holds state across requests.                                                                                                                                                                        |
| `vpn-broker-client-qbt`   | 431 | 1        | `modules/containers/qbittorrent/qbittorrent.nix`                                                              | Speaks the vpn-broker wire protocol plus the qBittorrent WebAPI.                                                                                                                                                                                                                |
| `qbt-scripts`             | 291 | 3        | `modules/containers/qbittorrent/qbittorrent.nix`, `modules/hosts/server/containers/qbittorrent/_exporter.nix` | `qbt-reannounce`, `qbt-watch`, `qbt-peer-exporter`. The exporter's JSON lines are parsed by Vector's `exec` source — the schema is a hard contract.                                                                                                                             |
| `vpn-broker-client-slskd` | 242 | 1        | `modules/containers/slskd/slskd.nix`                                                                          | vpn-broker protocol plus the slskd API.                                                                                                                                                                                                                                         |
| `tuwunel-init`            | 240 | 1        | `modules/containers/matrix/tuwunel.nix`                                                                       |                                                                                                                                                                                                                                                                                 |
| `soularr-reclaim`         | 216 | 1        | `modules/containers/soularr/soularr.nix`                                                                      | slskd plus Lidarr.                                                                                                                                                                                                                                                              |
| `ha-seeders`              | 165 | 3        | `modules/containers/home-assistant/home-assistant.nix`                                                        | `ha-seed-areas`, `ha-seed-config-entries`, `ha-seed-persons`. Each merges into Home Assistant's `.storage` JSON with its own semantics: areas append-only, persons never created, config entries sync identity fields and replace subentries. Reads seed data as JSON on stdin. |
| `nixos-notify`            | 120 | 1        | `modules/services/nixos-notify/nixos-notify.nix`                                                              | Called with the mosquitto binary path, broker IP, port and topic on argv.                                                                                                                                                                                                       |
| `hermes-resolve-rooms`    |  81 | 1        | `modules/containers/hermes/hermes.nix`                                                                        |                                                                                                                                                                                                                                                                                 |
| `qui-provision`           |  79 | 1        | `modules/containers/qui/qui.nix`                                                                              |                                                                                                                                                                                                                                                                                 |
| `forgejo-token-inject`    |  74 | 1        | `modules/hosts/server/containers/code-server/code-server.nix`                                                 | Referenced as `…/bin/forgejo-token-inject`.                                                                                                                                                                                                                                     |
| `thelounge-seed`          |  71 | 1        | `modules/containers/thelounge.nix`                                                                            |                                                                                                                                                                                                                                                                                 |

Every one of these imports only the Python standard library, except
`vpn-broker` (flask). Nothing here needs a Go dependency that
`lib/httpx` plus `encoding/json` does not already cover.

### Shell — ports

`writeShellApplication` pins each script's runtime dependencies; the Go port
reproduces that with `wrapProgram --prefix PATH`, listing the same tools.

| Project                     | LOC | Binaries | Runtime deps                                                                      | Consumer                                                                                                                                                                              |
| :-------------------------- | --: | :------- | :-------------------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `openrgb-init`              | 207 | 1        | gawk, gnugrep, coreutils                                                          | `modules/hw/corsair-dominator-titanium.nix`, `modules/hw/corsair-mm700.nix`, `modules/hw/nzxt-rgb-fan-controller.nix`                                                                 |
| `restic-backup`             | 170 | 2        | common + systemd (journalctl)                                                     | `modules/hosts/desktop/backups.nix`, `modules/hosts/server/backups.nix` — both reference `/bin/restic-backup` and `/bin/restic-backup-notify-fail`                                    |
| `audiobookshelf-setup`      | 133 | 1        | curl, jq, coreutils                                                               | `modules/containers/audiobookshelf.nix`                                                                                                                                               |
| `easyeffects-reload`        | 113 | 1        | pipewire, gnugrep, coreutils                                                      | `modules/apps/easyeffects/easyeffects.nix`                                                                                                                                            |
| `calibre-tools`             | 105 | 2        | coreutils, findutils                                                              | `modules/containers/calibre-web.nix` — `calibredb`/`ebook-convert` are deliberately **not** pinned; the caller supplies them on the unit's `path`, and the port must keep it that way |
| `komga-setup`               | 105 | 1        | curl, jq, coreutils                                                               | `modules/containers/komga.nix`                                                                                                                                                        |
| `kiwix-sync`                |  97 | 1        | rsync, gawk, gnugrep, findutils, coreutils                                        | `modules/containers/kiwix.nix`                                                                                                                                                        |
| `volume-notify`             |  90 | 1        | wireplumber (wpctl), pulseaudio (pactl), libnotify (notify-send), gawk, coreutils | `modules/services/volume-notifications.nix`                                                                                                                                           |
| `fscache-textfile`          |  56 | 1        | gawk, coreutils                                                                   | `modules/hosts/server/disks.nix`                                                                                                                                                      |
| `whippit`                   |  55 | 1        | whipper, coreutils, findutils, gnugrep                                            | `modules/cli/whipper.nix`                                                                                                                                                             |
| `coolercontrol-init-config` |  51 | 1        | coreutils                                                                         | `modules/hw/generic/coolercontrol.nix`                                                                                                                                                |

The ports replace `curl`/`jq` pipelines with `net/http` and `encoding/json`, so
those two drop out of the wrapper for the packages that only used them for API
calls. Tools that are genuinely external processes (rsync, wpctl, pactl,
notify-send, whipper, journalctl, openrgb) stay wrapped and are still exec'd.

## Waves

Waves order the work; each project inside a wave is still done on its own.

**Wave 0 — workspace prep.** Extend `mkProject` in `default.nix` with:

- optional `runtimeInputs`, adding `makeWrapper` to `nativeBuildInputs` and a
  `postInstall` that runs `wrapProgram --prefix PATH` over the module's
  binaries;
- optional `programs` (a list) for modules that install more than one binary,
  so `meta.mainProgram` stays correct and the wrapper loop knows what to wrap.

Nothing else in the workspace changes. This has to land before `flac-util` or
any shell port.

**Wave 1 — the four Go moves.** Mechanical: move the directory, module path rewrite,
`replace homelab => ../lib`, import rewrite, adopt `lib/app` for bootstrap and
`lib/buildinfo` for the version stamp (dropping any per-binary `version` var),
fold hand-rolled HTTP onto `lib/httpx` where it is a straight substitution.
Behaviour must not move at all; these land first because they carry no
translation risk and they prove the wave-0 wrapper support.

**Wave 2 — small and mid Python.** In rough ascending order of risk:
`thelounge-seed`, `forgejo-token-inject`, `qui-provision`,
`hermes-resolve-rooms`, `nixos-notify`, `tuwunel-init`, `soularr-reclaim`,
`ha-seeders`, `qbt-scripts`.

**Wave 3 — `beets-lidarr-import`.** Largest Python port, and the only one with
a test suite to port alongside it.

**Wave 4 — the vpn-broker trio.** `vpn-broker`, `vpn-broker-client-qbt` and
`vpn-broker-client-slskd` share a wire protocol, so they move together: the
protocol types and client go to `lib/clients/vpnbroker`, the server lives in
`vpn-broker/internal/`. Three projects, but gated as one unit, because a
protocol change in one is a change in all three.

**Wave 5 — shell.** In rough ascending order of risk:
`coolercontrol-init-config`, `fscache-textfile`, `whippit`, `volume-notify`,
`kiwix-sync`, `komga-setup`, `easyeffects-reload`, `audiobookshelf-setup`,
`calibre-tools`, `restic-backup`, `openrgb-init`.

## Requirements

1. **Behavioural parity, no new features.** Same flags and flag names, same
   argv order, same environment variables, same exit codes, same stdout
   format, same files touched. No flag, guard, retry or option that the
   original did not have. Where the original's behaviour is surprising, the
   port reproduces the surprise and records it in a comment.
2. **Names do not change.** Every `pkgs.homelab.<name>` attribute and every
   binary name survives the move, so no file under `modules/` needs editing for
   a port. Comments and docs that name the old `packages/<name>` path do get
   updated: `modules/overlays.nix`, `modules/formatter.nix`,
   `modules/containers/mousetrap/mousetrap.nix`, `modules/cli/whipper.nix`,
   `packages/mousetrap/README.md`.
3. **Layout per `README.md`.** `module homelab/<name>` with
   `replace homelab => ../lib`; `main.go` declares the flags, builds a config
   struct and hands off; all logic under `internal/<pkg>/`; `cmd/<name>/` only
   for the four multi-binary projects.
4. **Use the shared library.** `lib/app` for bootstrap, flags, signals and exit
   codes; `lib/config`; `lib/obs` for logging and metrics; `lib/httpx` for HTTP
   clients; `lib/secret` for credentials, every one as a `--x` / `--x-file`
   pair so the host decides what is secret. New per-service API clients go to
   `lib/clients/<service>/`; several already exist (arr, qbittorrent, slskd,
   prowlarr, autobrr, bazarr, beets, crossseed).
5. **stdout is the product, stderr is the log.** `forbidigo` rejects
   `fmt.Println`. Anything the original printed as progress becomes a log
   record; anything a caller parses stays on stdout byte-identical.
6. **Coverage floor 80% on every non-`main` package.** This is the largest
   single cost of the plan: most of the Python and all of the shell ships no
   tests today. Budget test writing as the bulk of each wave-2 and wave-5
   port, not an afterthought.
7. **External processes stay wrapped.** Each project's `default.nix` entry
   lists its `runtimeInputs`; nothing is assumed to be on PATH. `calibre-tools`
   is the deliberate exception and stays one.
8. **Vector and systemd contracts are frozen.** `qbt-peer-exporter`'s JSON line
   schema, `restic-backup-notify-fail`'s journal reading and any systemd
   notification protocol the originals use must be preserved exactly; these are
   consumed by configuration that this work does not touch.
9. **Every observable surface is audited, not assumed.** A port changes more
   than the source tree, and "nothing changed" has to be a measurement. For
   each project, work through the surfaces in the checklist below: the
   container's `ExecStart`, the flag set, the exported metrics, the dashboards
   and log queries that read them, and the log destination. `lib/app` adds
   flags and metrics of its own — that is a change to the deployed surface even
   when the tool's own behaviour is identical, and it belongs in the handover
   note rather than being discovered on the next scrape.

## Per-project checklist

1. Write the Go project under `packages/go/<name>/` (`go.mod` declaring
   `module homelab/<name>` with `replace homelab => ../lib`).
2. Add `./<name>` to `go.work`.
3. Add the `mkProject` entry to `default.nix`, with `runtimeInputs` and
   `programs` where needed.
4. Add a placeholder line `<name> = "";` to `vendor-hashes.nix` —
   `just go-hashes` rewrites an existing line, it does not create one.
5. `just format` — it stages new files first, which Nix needs because flakes
   ignore untracked ones.
6. `just go-hashes`, then `just go` (lint, `-race` tests, coverage floor).
7. `nix fmt`.
8. Delete the old `packages/<name>/` directory; update any comment that names
   the old path.
9. `just flake` (module set changed), then `just eval-server` /
   `just eval-desktop` and the relevant `just build-*`.
10. `just topology` if anything under `modules/infra` or a container changed —
    CI fails on a stale README.
11. Audit the deployed surface (requirement 9). See below.
12. Hand the `nixos-rebuild switch`/`test` to the user. Never run it here.

## Surface audit

Run this for every project that something else consumes — a container, a
timer, a scrape, a dashboard. Each step is a command whose output is the
evidence, not a judgement call.

1. **The container runs the new build.** Compare the built path with what the
   unit will execute; they must be the same store path:

   ```
   nix build .#<name> --no-link --print-out-paths
   nix eval --raw .#nixosConfigurations.server.config.containers.<c>.config.systemd.services.<s>.serviceConfig.ExecStart
   ```

   An attribute that kept its name needs no `modules/` edit, and this is how
   that is confirmed rather than assumed.

2. **The real argv still parses.** Take that `ExecStart` verbatim, redirect
   only the bind address, state directory and any secret path, and run it.
   A flag the port renamed or dropped fails here, before deployment. Check
   `/health`, one route that needs no external service, and that the process
   exits 0 on `SIGTERM`.

3. **The flag surface is diffed, both ways.** Every flag the unit passes must
   reach its field (a mapping test asserts this), and every flag `lib/app` adds
   — `--config`, `--log.*`, `--metrics.*`, `--dry-run`, `--help`, `--version` —
   is new vocabulary the unit does not pass but now accepts. Also check the
   unit for `HOMELAB_*` environment variables, which `lib/config` now reads as
   a configuration layer.

4. **The exported metrics are diffed.** Scrape the new binary and list the
   families:

   ```
   curl -fsS http://127.0.0.1:<port>/metrics | grep -oE '^[a-z_]+' | sort -u
   ```

   A tool that moves onto `lib/obs`'s registry gains `go_*` and `process_*`
   whether or not it asks for them. Additions are fine; a rename or a
   disappearance is a dashboard break. Series that only exist once the ledgers
   have rows will be missing from an empty instance — compare against the
   collector source, not only against a cold scrape.

5. **The dashboards that read them still resolve.** Extract the metric names a
   dashboard references and compare against what the code produces:

   ```
   grep -ohE '(<tool>|<prefix>)_[a-z_]+' modules/hosts/server/containers/grafana/dashboards/*.nix | sort -u
   ```

   Expect histogram suffixes (`_bucket`, `_count`, `_sum`) and prose mentions
   to show up as unmatched; everything else must exist.

6. **The log queries still match.** Moving a tool onto `lib/obs` moves its logs
   to stderr and adds `service` and `version` attributes. Check every Loki
   panel for a `stream` selector and for filters on `msg` values the port may
   have reworded, and fix the comments that describe the stream.

## Known risks

- **`nixos-notify`** is a desktop notification path. Going through D-Bus adds a
  Go dependency; exec'ing `notify-send` keeps the wrapper honest but changes
  the process tree. Decide per the original's actual mechanism when the port
  starts, and prefer whichever preserves observable behaviour.
- **`vpn-broker`** keeps state across requests under Flask's threading model.
  The Go port needs explicit synchronisation, and a race here is invisible
  until traffic hits it. Its tests should exercise concurrent requests.
- **`ha-seeders`** mutates Home Assistant's `.storage` files in place. A merge
  bug corrupts a live registry. Port each seeder against a golden fixture of
  the real file shapes before switching the container over.
- **`flac-util`** is 19k lines of DSP. The move is pure rename plus import
  rewrite, but the diff is large enough to hide an accident — verify by
  comparing analysis output on a fixed sample set before and after.
- **Shell semantics.** `calibre-ingest` runs with `nounset` only, deliberately
  relying on individual file failures being skipped rather than aborting the
  batch. Several other scripts depend on exit codes of piped tools. Read each
  script's `bashOptions` and error handling before porting the loop.
