# overlays — auto-register every packages/<name> under pkgs.homelab.<name>.
# Discovers the subdirectories of ../packages and callPackages each into a single
# `homelab` namespace, so a custom package is referenced anywhere as
# `pkgs.homelab.<name>` (e.g. `lib.getExe pkgs.homelab.sys-mcp`) instead of a
# relative `pkgs.callPackage ../../packages/<name> { }`. The namespace avoids
# shadowing any top-level nixpkgs attribute. Adding a new package is just a new
# directory — no per-package wiring. New dirs must be git-tracked before Nix can
# see them (flakes ignore untracked files).
{ lib, ... }:
let
  pkgsDir = ../packages;
  overlay = _final: prev: {
    homelab = lib.mapAttrs (name: _: prev.callPackage (pkgsDir + "/${name}") { }) (
      lib.filterAttrs (_: type: type == "directory") (builtins.readDir pkgsDir)
    );
  };
in
{
  den.default.nixos.nixpkgs.overlays = [ overlay ];
  den.default.darwin.nixpkgs.overlays = [ overlay ];
}
