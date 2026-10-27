# overlays — auto-register every packages/<name> under pkgs.homelab.<name>.
# Discovers the subdirectories of ../packages and callPackages each into a single
# `homelab` namespace, so a custom package is referenced anywhere as
# `pkgs.homelab.<name>` (e.g. `lib.getExe pkgs.homelab.sys-mcp`) instead of a
# relative `pkgs.callPackage ../../packages/<name> { }`. The namespace avoids
# shadowing any top-level nixpkgs attribute. The same set is re-exported as flake
# package outputs, so `nix build .#<name>` works too. Adding a new package is just
# a new directory — no per-package wiring. New dirs must be git-tracked before Nix
# can see them (flakes ignore untracked files).
{ lib, ... }:
let
  pkgsDir = ../packages;
  names = lib.filterAttrs (_: type: type == "directory") (builtins.readDir pkgsDir);

  # A package directory normally evaluates to a single derivation, registered
  # under the directory's own name. packages/go is the exception: it holds a Go
  # workspace of several modules and evaluates to an attribute set of
  # derivations instead. Those merge into the namespace flat rather than
  # nesting, so a binary that moves into the workspace is still reachable as
  # pkgs.homelab.<name> and no callsite has to know where it lives.
  # callPackage runs its result through lib.makeOverridable, which decorates a
  # returned attribute set with `override` and `overrideDerivation`. Those are
  # meaningful on a single package but are not packages themselves, so the
  # multi-derivation case keeps only the derivations and drops the machinery.
  callOne = pkgs: name: pkgs.callPackage (pkgsDir + "/${name}") { };
  expand =
    name: value:
    if lib.isDerivation value then
      { ${name} = value; }
    else
      lib.filterAttrs (_: lib.isDerivation) value;
  callAll =
    pkgs: lib.foldl' (acc: name: acc // expand name (callOne pkgs name)) { } (lib.attrNames names);
  overlay = _final: prev: { homelab = callAll prev; };
in
{
  den.default.nixos.nixpkgs.overlays = [ overlay ];
  den.default.darwin.nixpkgs.overlays = [ overlay ];

  # `pkgs` here already has the overlay applied, so these are the very same
  # derivations as pkgs.homelab.<name> — not a second evaluation.
  perSystem =
    { pkgs, ... }:
    {
      packages = callAll pkgs;
    };
}
