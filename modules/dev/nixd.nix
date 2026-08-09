/*
  nixd — Nix language server with nixpkgs/flake-aware completion and diagnostics
  https://github.com/nix-community/nixd
*/
_: {
  den.aspects.dev._.nixd = {
    aspect-name = "dev.nixd";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.nixd ];
      };
  };
}
