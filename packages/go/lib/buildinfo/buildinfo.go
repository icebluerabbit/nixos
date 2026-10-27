// Package buildinfo carries the version stamped into every homelab binary.
//
// One variable for the whole workspace rather than a main.version per binary:
// the tools module is compiled as a unit, so a single -X ldflag in
// packages/go/default.nix stamps every command in it at once.
//
// Nix builds have no VCS metadata — the source arrives as a store path, not a
// checkout — so debug.ReadBuildInfo cannot supply this and the ldflag is the
// only route.
package buildinfo

// Version is set at link time with:
//
//	-X homelab/buildinfo.Version=<version>
//
// It stays "dev" for `go run` and `go build` during development, which is the
// honest answer: those binaries were not built from a pinned source.
var Version = "dev"
