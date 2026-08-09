/*
  nh — Nix Helper; ergonomic wrapper around nixos-rebuild and home-manager with better output and dry-run support
  https://github.com/viperML/nh
*/
_: {
  den.aspects.cli._.nh = {
    aspect-name = "cli.nh";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.nh ];
      };
  };
}
