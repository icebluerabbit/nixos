# tools

Every homelab binary that is not big enough to deserve its own module. One
directory per binary under `cmd/`, where the directory name is simultaneously
the Go package, the binary name, and the `pkgs.homelab.<name>` attribute.

```
tools/
  cmd/
    topology/main.go     -> pkgs.homelab.topology
    komga-setup/main.go  -> pkgs.homelab.komga-setup
  internal/              shared by a couple of tools, not yet lib-worthy
```

## Rules

- **`main.go` holds no logic.** Flags, config load, construct, call, exit —
  around 30 lines. Behaviour lives in `homelab` (`lib/`) or `homelab/clients`
  where `go test ./...` reaches it without spawning a process.
- **`internal/` is a holding pen.** Helpers shared by two tools live here.
  Promote to `lib/` at the third consumer; invented abstractions with fewer
  than three consumers stay duplicated.
- **One module, one `vendorHash`.** All of these share a dependency set that is
  close to empty, which is what keeps roughly two dozen binaries tractable to
  build. A tool that needs a heavy dependency gets its own module instead.

## Why `cmd/` is excluded from coverage

`main()` is the one place that cannot be unit-tested without spawning a
process, and it is kept trivial precisely so that not testing it costs nothing.
Coverage is enforced on `lib/` and `clients/`.
